# GoImageFinder → SvelteKit Cloud Architecture

## Overview

This document outlines the architecture for porting the GoImageFinder application to a SvelteKit-based cloud application with multi-user support, external disk management, and image similarity search capabilities.

---

## Table of Contents

1. [Technology Stack](#technology-stack)
2. [Server Requirements & Installation](#server-requirements--installation)
3. [Database Design: PostgreSQL vs Redis](#database-design-postgresql-vs-redis)
4. [Database Schema](#database-schema)
5. [Application Architecture](#application-architecture)
6. [Go CLI Integration Strategy](#go-cli-integration-strategy)
7. [File Storage & Thumbnails](#file-storage--thumbnails)
8. [Authentication & Authorization](#authentication--authorization)
9. [API Design](#api-design)
10. [Deployment Architecture](#deployment-architecture)
11. [Reverse Proxy: Caddy vs Nginx](#reverse-proxy-caddy-vs-nginx)
12. [Scalability & Horizontal Scaling](#scalability--horizontal-scaling)
13. [Subscription & Payment System](#subscription--payment-system)
14. [Expanded Vision & Future Features](#expanded-vision--future-features)

---

## Technology Stack

### Frontend & Backend
- **Framework:** SvelteKit 2 + Svelte 5
- **Runtime:** Bun (fast JavaScript runtime)
- **Styling:** Tailwind CSS 4
- **Forms:** sveltekit-superforms + Zod validation
- **Testing:** Playwright (E2E)

### Database & Cache
- **Primary Database:** PostgreSQL (relational data, user management, disk/image metadata)
- **Cache/Queue:** Redis (session storage, scan job queues, real-time progress, rate limiting)

### Image Processing
- **Go CLI:** Compiled `goimagefinder` binary for image scanning/hashing
- **Thumbnail Storage:** S3-compatible storage (MinIO for self-hosted, AWS S3 for cloud)

---

## Server Requirements & Installation

### Minimum Server Specifications

```
CPU:        4 cores (8+ recommended for heavy scanning)
RAM:        8 GB (16+ GB recommended)
Storage:    100 GB SSD (for OS, database, thumbnails)
OS:         Ubuntu 22.04 LTS or Debian 12
```

### Required Software Installation

```bash
#!/bin/bash
# server-setup.sh

# 1. System Updates
sudo apt update && sudo apt upgrade -y

# 2. Install Essential Tools
sudo apt install -y curl wget git build-essential unzip

# 3. Install Bun Runtime
curl -fsSL https://bun.sh/install | bash
source ~/.bashrc

# 4. Install PostgreSQL 16
sudo sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo apt-key add -
sudo apt update
sudo apt install -y postgresql-16 postgresql-contrib-16

# 5. Install Redis 7
sudo apt install -y redis-server
sudo systemctl enable redis-server

# 6. Install OpenCV dependencies (for Go image processor)
sudo apt install -y libopencv-dev pkg-config

# 7. Install ExifTool (for RAW metadata extraction)
sudo apt install -y libimage-exiftool-perl

# 8. Install Go 1.24+ (for building goimagefinder)
wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 9. Install MinIO (S3-compatible object storage for thumbnails)
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio
sudo mv minio /usr/local/bin/

# 10. Install Nginx (reverse proxy)
sudo apt install -y nginx certbot python3-certbot-nginx

# 11. Install PM2 (process manager via npm/bun)
bun install -g pm2
```

### PostgreSQL Setup

```bash
# Create database and user
sudo -u postgres psql <<EOF
CREATE USER goimagefinder WITH PASSWORD 'secure_password_here';
CREATE DATABASE goimagefinder_db OWNER goimagefinder;
GRANT ALL PRIVILEGES ON DATABASE goimagefinder_db TO goimagefinder;
\c goimagefinder_db
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";  -- For text search
EOF
```

### Redis Configuration

```bash
# /etc/redis/redis.conf modifications
sudo sed -i 's/^# maxmemory .*/maxmemory 256mb/' /etc/redis/redis.conf
sudo sed -i 's/^# maxmemory-policy .*/maxmemory-policy allkeys-lru/' /etc/redis/redis.conf
sudo systemctl restart redis-server
```

### Build Go CLI Binary

```bash
# Clone and build goimagefinder
cd /opt
git clone <your-repo> goimagefinder
cd goimagefinder
make build

# Make binary available system-wide
sudo cp build/goimagefinder /usr/local/bin/
sudo chmod +x /usr/local/bin/goimagefinder
```

---

## Database Design: PostgreSQL vs Redis

### Recommendation: Use BOTH (Hybrid Approach)

| Use Case | PostgreSQL | Redis |
|----------|------------|-------|
| User accounts & authentication | ✅ Primary | Session tokens |
| Disk/folder metadata | ✅ Primary | — |
| Image records & hashes | ✅ Primary | Cache hot queries |
| Scan job queue | — | ✅ Primary (Bull/BullMQ) |
| Real-time scan progress | — | ✅ Pub/Sub |
| Rate limiting | — | ✅ Primary |
| Search results cache | — | ✅ TTL cache |
| Session storage | — | ✅ Primary |

### Why PostgreSQL for Core Data

1. **ACID Compliance:** Critical for user data, billing, and audit trails
2. **Complex Queries:** JOINs across users, disks, images with filtering
3. **Data Integrity:** Foreign keys ensure referential integrity
4. **Full-Text Search:** Native support with `pg_trgm` for filename search
5. **Indexing:** B-tree indexes on hashes enable fast similarity lookups
6. **Drizzle ORM:** Excellent PostgreSQL support with type safety

### Why Redis for Real-Time & Caching

1. **Scan Job Queue:** BullMQ for reliable job processing with retries
2. **Progress Tracking:** Pub/Sub for real-time UI updates during scans
3. **Session Storage:** Fast, TTL-based session management
4. **Rate Limiting:** Sliding window counters for API protection
5. **Search Cache:** Cache similarity results (5-minute TTL)
6. **Distributed Locks:** Prevent concurrent scans on same disk

### When to Consider Redis as Primary (Alternative)

If your use case is:
- Read-heavy with simple key-value lookups
- Ephemeral data that can be regenerated
- No complex relationships between entities
- Sub-millisecond latency requirements

**For this application:** PostgreSQL is better for the core data model due to relational nature of users → disks → images hierarchy.

---

## Database Schema

### Drizzle ORM Schema (PostgreSQL)

```typescript
// src/lib/server/db/schema.ts

import { pgTable, text, timestamp, uuid, integer, boolean, index, uniqueIndex } from 'drizzle-orm/pg-core';
import { relations } from 'drizzle-orm';

// ============================================
// USERS TABLE
// ============================================
export const users = pgTable('users', {
  id: uuid('id').defaultRandom().primaryKey(),
  email: text('email').notNull().unique(),
  passwordHash: text('password_hash').notNull(),
  name: text('name'),
  avatarUrl: text('avatar_url'),
  emailVerified: boolean('email_verified').default(false),
  createdAt: timestamp('created_at').defaultNow().notNull(),
  updatedAt: timestamp('updated_at').defaultNow().notNull(),
  lastLoginAt: timestamp('last_login_at'),
  // Subscription/limits
  tier: text('tier').default('free').notNull(), // 'free', 'pro', 'enterprise'
  maxDisks: integer('max_disks').default(3),
  maxImagesPerDisk: integer('max_images_per_disk').default(10000),
}, (table) => ({
  emailIdx: uniqueIndex('users_email_idx').on(table.email),
}));

// ============================================
// SESSIONS TABLE (for auth)
// ============================================
export const sessions = pgTable('sessions', {
  id: text('id').primaryKey(), // Session token
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
  expiresAt: timestamp('expires_at').notNull(),
  createdAt: timestamp('created_at').defaultNow().notNull(),
  userAgent: text('user_agent'),
  ipAddress: text('ip_address'),
});

// ============================================
// DISKS TABLE
// ============================================
export const disks = pgTable('disks', {
  id: uuid('id').defaultRandom().primaryKey(),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),

  // User-defined metadata
  name: text('name').notNull(),              // e.g., "MacBook Pro SSD"
  description: text('description'),          // Optional notes
  diskIdentifier: text('disk_identifier'),   // Unique disk serial/UUID
  mountPath: text('mount_path'),             // e.g., "/Volumes/MyDisk"

  // Scan configuration
  scanPaths: text('scan_paths').array(),     // Folders to scan on this disk
  excludePatterns: text('exclude_patterns').array(), // Glob patterns to exclude

  // Status tracking
  status: text('status').default('idle').notNull(), // 'idle', 'scanning', 'error'
  lastScanAt: timestamp('last_scan_at'),
  lastScanDuration: integer('last_scan_duration'),  // Seconds
  totalImages: integer('total_images').default(0),
  totalSize: integer('total_size').default(0),      // Bytes

  // Timestamps
  createdAt: timestamp('created_at').defaultNow().notNull(),
  updatedAt: timestamp('updated_at').defaultNow().notNull(),
}, (table) => ({
  userIdx: index('disks_user_idx').on(table.userId),
  nameIdx: index('disks_name_idx').on(table.userId, table.name),
}));

// ============================================
// IMAGES TABLE
// ============================================
export const images = pgTable('images', {
  id: uuid('id').defaultRandom().primaryKey(),
  diskId: uuid('disk_id').notNull().references(() => disks.id, { onDelete: 'cascade' }),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),

  // File information
  path: text('path').notNull(),              // Full path on disk
  relativePath: text('relative_path'),       // Path relative to scan root
  filename: text('filename').notNull(),
  format: text('format'),                    // jpeg, png, cr2, etc.

  // Image metadata
  width: integer('width'),
  height: integer('height'),
  fileSize: integer('file_size'),            // Bytes
  fileModifiedAt: timestamp('file_modified_at'),

  // Perceptual hashes (from Go CLI)
  averageHash: text('average_hash'),         // 64-bit hex string
  perceptualHash: text('perceptual_hash'),   // 1024-bit hex string

  // Thumbnail reference
  thumbnailKey: text('thumbnail_key'),       // S3/MinIO object key
  thumbnailGenerated: boolean('thumbnail_generated').default(false),

  // Processing status
  processingStatus: text('processing_status').default('pending'), // 'pending', 'processed', 'error'
  processingError: text('processing_error'),

  // Timestamps
  createdAt: timestamp('created_at').defaultNow().notNull(),
  updatedAt: timestamp('updated_at').defaultNow().notNull(),
}, (table) => ({
  diskIdx: index('images_disk_idx').on(table.diskId),
  userIdx: index('images_user_idx').on(table.userId),
  pathIdx: uniqueIndex('images_path_idx').on(table.diskId, table.path),
  avgHashIdx: index('images_avg_hash_idx').on(table.averageHash),
  pHashIdx: index('images_phash_idx').on(table.perceptualHash),
  formatIdx: index('images_format_idx').on(table.format),
}));

// ============================================
// SCAN JOBS TABLE (for tracking/history)
// ============================================
export const scanJobs = pgTable('scan_jobs', {
  id: uuid('id').defaultRandom().primaryKey(),
  diskId: uuid('disk_id').notNull().references(() => disks.id, { onDelete: 'cascade' }),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),

  status: text('status').default('queued').notNull(), // 'queued', 'running', 'completed', 'failed', 'cancelled'
  progress: integer('progress').default(0),           // 0-100
  currentFile: text('current_file'),

  // Results
  totalFiles: integer('total_files').default(0),
  processedFiles: integer('processed_files').default(0),
  skippedFiles: integer('skipped_files').default(0),
  errorFiles: integer('error_files').default(0),
  newImages: integer('new_images').default(0),

  // Timing
  startedAt: timestamp('started_at'),
  completedAt: timestamp('completed_at'),

  // Error handling
  errorMessage: text('error_message'),

  createdAt: timestamp('created_at').defaultNow().notNull(),
}, (table) => ({
  diskIdx: index('scan_jobs_disk_idx').on(table.diskId),
  statusIdx: index('scan_jobs_status_idx').on(table.status),
}));

// ============================================
// SEARCH HISTORY TABLE (optional, for analytics)
// ============================================
export const searchHistory = pgTable('search_history', {
  id: uuid('id').defaultRandom().primaryKey(),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),

  queryImagePath: text('query_image_path'),
  queryImageHash: text('query_image_hash'),
  threshold: integer('threshold'),            // Stored as int (0-100)
  resultsCount: integer('results_count'),
  searchDuration: integer('search_duration'), // Milliseconds

  createdAt: timestamp('created_at').defaultNow().notNull(),
});

// ============================================
// RELATIONS
// ============================================
export const usersRelations = relations(users, ({ many }) => ({
  disks: many(disks),
  images: many(images),
  sessions: many(sessions),
  scanJobs: many(scanJobs),
  searchHistory: many(searchHistory),
}));

export const disksRelations = relations(disks, ({ one, many }) => ({
  user: one(users, { fields: [disks.userId], references: [users.id] }),
  images: many(images),
  scanJobs: many(scanJobs),
}));

export const imagesRelations = relations(images, ({ one }) => ({
  disk: one(disks, { fields: [images.diskId], references: [disks.id] }),
  user: one(users, { fields: [images.userId], references: [users.id] }),
}));
```

---

## Application Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CLOUD SERVER                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐  │
│  │   SvelteKit App   │    │   Go CLI Binary   │    │     Worker       │  │
│  │   (Bun Runtime)   │◄──►│  (goimagefinder)  │◄──►│   (BullMQ)       │  │
│  │                   │    │                   │    │                   │  │
│  │  - Auth/Sessions  │    │  - scan command   │    │  - Job processor │  │
│  │  - REST API       │    │  - search command │    │  - Progress pub  │  │
│  │  - SSE endpoints  │    │  - thumbnail gen  │    │  - Error retry   │  │
│  │  - UI serving     │    │                   │    │                   │  │
│  └────────┬─────────┘    └──────────────────┘    └────────┬─────────┘  │
│           │                                                 │            │
│           │              ┌──────────────────┐               │            │
│           └─────────────►│     Redis        │◄──────────────┘            │
│                          │                   │                           │
│                          │  - Session store  │                           │
│                          │  - Job queue      │                           │
│                          │  - Pub/Sub        │                           │
│                          │  - Cache          │                           │
│                          └──────────────────┘                           │
│                                   │                                      │
│           ┌───────────────────────┼───────────────────────┐             │
│           │                       │                       │             │
│           ▼                       ▼                       ▼             │
│  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐  │
│  │   PostgreSQL     │    │     MinIO        │    │  External Disks  │  │
│  │                   │    │   (S3-compat)    │    │                   │  │
│  │  - Users          │    │                   │    │  /mnt/disk1      │  │
│  │  - Disks          │    │  - Thumbnails    │    │  /mnt/disk2      │  │
│  │  - Images         │    │  - Exports       │    │  /media/usb      │  │
│  │  - Scan history   │    │                   │    │                   │  │
│  └──────────────────┘    └──────────────────┘    └──────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ HTTPS
                                    ▼
                            ┌──────────────────┐
                            │     Nginx        │
                            │  (Reverse Proxy) │
                            │  + Let's Encrypt │
                            └──────────────────┘
                                    │
                                    ▼
                            ┌──────────────────┐
                            │     Users        │
                            │   (Web Browser)  │
                            └──────────────────┘
```

### Directory Structure

```
sveltekit-imagefinder/
├── src/
│   ├── lib/
│   │   ├── components/           # Svelte 5 components
│   │   │   ├── ui/              # Shadcn-svelte components
│   │   │   ├── DiskTable.svelte
│   │   │   ├── ImageGrid.svelte
│   │   │   ├── ScanProgress.svelte
│   │   │   └── SearchResults.svelte
│   │   │
│   │   ├── server/              # Server-only code
│   │   │   ├── db/
│   │   │   │   ├── index.ts     # Drizzle client
│   │   │   │   └── schema.ts    # Database schema
│   │   │   ├── auth/
│   │   │   │   ├── lucia.ts     # Auth configuration
│   │   │   │   └── password.ts  # Password hashing
│   │   │   ├── redis/
│   │   │   │   ├── client.ts    # Redis connection
│   │   │   │   └── queue.ts     # BullMQ setup
│   │   │   ├── storage/
│   │   │   │   └── s3.ts        # MinIO/S3 client
│   │   │   └── cli/
│   │   │       └── goimagefinder.ts  # Go CLI wrapper
│   │   │
│   │   ├── schemas/             # Zod validation schemas
│   │   │   ├── auth.ts
│   │   │   ├── disk.ts
│   │   │   └── search.ts
│   │   │
│   │   └── utils/               # Shared utilities
│   │
│   ├── routes/
│   │   ├── +layout.svelte
│   │   ├── +page.svelte         # Landing/dashboard
│   │   │
│   │   ├── auth/
│   │   │   ├── login/+page.svelte
│   │   │   ├── register/+page.svelte
│   │   │   └── logout/+server.ts
│   │   │
│   │   ├── dashboard/
│   │   │   ├── +page.svelte     # User dashboard
│   │   │   ├── disks/
│   │   │   │   ├── +page.svelte # Disk management
│   │   │   │   └── [diskId]/
│   │   │   │       ├── +page.svelte  # Disk details
│   │   │   │       └── images/+page.svelte
│   │   │   └── search/
│   │   │       └── +page.svelte # Search interface
│   │   │
│   │   └── api/
│   │       ├── disks/
│   │       │   ├── +server.ts   # CRUD operations
│   │       │   └── [diskId]/
│   │       │       ├── +server.ts
│   │       │       └── scan/+server.ts
│   │       ├── images/
│   │       │   ├── +server.ts
│   │       │   └── [imageId]/thumbnail/+server.ts
│   │       ├── search/
│   │       │   └── +server.ts
│   │       └── scan-progress/
│   │           └── +server.ts   # SSE endpoint
│   │
│   ├── hooks.server.ts          # Auth middleware
│   └── app.d.ts                 # Type declarations
│
├── workers/
│   └── scan-worker.ts           # BullMQ job processor
│
├── bin/
│   └── goimagefinder            # Compiled Go binary
│
├── drizzle/
│   └── migrations/              # Database migrations
│
├── tests/
│   └── e2e/                     # Playwright tests
│
├── drizzle.config.ts
├── svelte.config.js
├── tailwind.config.ts
├── package.json
└── docker-compose.yml
```

---

## Go CLI Integration Strategy

### Wrapper Module

```typescript
// src/lib/server/cli/goimagefinder.ts

import { spawn, type ChildProcess } from 'child_process';
import { EventEmitter } from 'events';

const CLI_PATH = process.env.GOIMAGEFINDER_PATH || '/usr/local/bin/goimagefinder';

export interface ScanOptions {
  folder: string;
  database: string;
  prefix?: string;
  force?: boolean;
  debug?: boolean;
}

export interface ScanProgress {
  type: 'progress' | 'complete' | 'error';
  currentFile?: string;
  processed?: number;
  total?: number;
  message?: string;
}

export interface SearchResult {
  path: string;
  similarity: number;
  width: number;
  height: number;
  format: string;
}

export class GoImageFinder extends EventEmitter {
  private process: ChildProcess | null = null;

  /**
   * Execute scan command and stream progress
   */
  async scan(options: ScanOptions): Promise<void> {
    const args = [
      'scan',
      `--folder=${options.folder}`,
      `--database=${options.database}`,
    ];

    if (options.prefix) args.push(`--prefix=${options.prefix}`);
    if (options.force) args.push('--force');
    if (options.debug) args.push('--debug');

    return new Promise((resolve, reject) => {
      this.process = spawn(CLI_PATH, args);

      this.process.stdout?.on('data', (data: Buffer) => {
        const lines = data.toString().split('\n');
        for (const line of lines) {
          if (line.includes('Processing:')) {
            const match = line.match(/Processing: (.+)/);
            if (match) {
              this.emit('progress', {
                type: 'progress',
                currentFile: match[1],
              } as ScanProgress);
            }
          }
        }
      });

      this.process.stderr?.on('data', (data: Buffer) => {
        this.emit('error', { type: 'error', message: data.toString() });
      });

      this.process.on('close', (code) => {
        if (code === 0) {
          this.emit('complete', { type: 'complete' });
          resolve();
        } else {
          reject(new Error(`Process exited with code ${code}`));
        }
      });
    });
  }

  /**
   * Execute search command
   */
  async search(
    imagePath: string,
    database: string,
    threshold: number = 0.8,
    prefix?: string
  ): Promise<SearchResult[]> {
    const args = [
      'search',
      `--image=${imagePath}`,
      `--database=${database}`,
      `--threshold=${threshold}`,
    ];

    if (prefix) args.push(`--prefix=${prefix}`);

    return new Promise((resolve, reject) => {
      const process = spawn(CLI_PATH, args);
      let stdout = '';
      let stderr = '';

      process.stdout?.on('data', (data: Buffer) => {
        stdout += data.toString();
      });

      process.stderr?.on('data', (data: Buffer) => {
        stderr += data.toString();
      });

      process.on('close', (code) => {
        if (code === 0) {
          // Parse JSON output from CLI
          try {
            const results = this.parseSearchResults(stdout);
            resolve(results);
          } catch (e) {
            reject(new Error(`Failed to parse results: ${e}`));
          }
        } else {
          reject(new Error(stderr || `Process exited with code ${code}`));
        }
      });
    });
  }

  /**
   * Cancel running scan
   */
  cancel(): void {
    if (this.process) {
      this.process.kill('SIGTERM');
      this.process = null;
    }
  }

  private parseSearchResults(output: string): SearchResult[] {
    // Parse CLI output format
    const results: SearchResult[] = [];
    const lines = output.split('\n');

    for (const line of lines) {
      const match = line.match(/Path: (.+), Similarity: ([\d.]+)/);
      if (match) {
        results.push({
          path: match[1],
          similarity: parseFloat(match[2]),
          width: 0,
          height: 0,
          format: '',
        });
      }
    }

    return results;
  }
}

// Singleton for job worker
export const goimagefinder = new GoImageFinder();
```

### BullMQ Scan Worker

```typescript
// workers/scan-worker.ts

import { Worker, Job } from 'bullmq';
import { Redis } from 'ioredis';
import { GoImageFinder } from '../src/lib/server/cli/goimagefinder';
import { db } from '../src/lib/server/db';
import { disks, images, scanJobs } from '../src/lib/server/db/schema';
import { eq } from 'drizzle-orm';

const redis = new Redis(process.env.REDIS_URL || 'redis://localhost:6379');

interface ScanJobData {
  jobId: string;
  diskId: string;
  userId: string;
  scanPath: string;
  force: boolean;
}

const worker = new Worker<ScanJobData>(
  'scan-queue',
  async (job: Job<ScanJobData>) => {
    const { jobId, diskId, userId, scanPath, force } = job.data;
    const cli = new GoImageFinder();

    // Update job status to running
    await db.update(scanJobs)
      .set({ status: 'running', startedAt: new Date() })
      .where(eq(scanJobs.id, jobId));

    // Update disk status
    await db.update(disks)
      .set({ status: 'scanning' })
      .where(eq(disks.id, diskId));

    // Temporary SQLite database for Go CLI
    const tempDbPath = `/tmp/scan_${jobId}.db`;

    // Listen to progress and publish to Redis
    cli.on('progress', async (progress) => {
      await redis.publish(`scan:${jobId}`, JSON.stringify(progress));
      await job.updateProgress(progress);
    });

    try {
      // Run the Go CLI scan
      await cli.scan({
        folder: scanPath,
        database: tempDbPath,
        prefix: diskId,
        force,
        debug: false,
      });

      // Import results from temp SQLite to PostgreSQL
      await importScanResults(tempDbPath, diskId, userId);

      // Update job as completed
      await db.update(scanJobs)
        .set({
          status: 'completed',
          completedAt: new Date(),
          progress: 100,
        })
        .where(eq(scanJobs.id, jobId));

      // Update disk status
      await db.update(disks)
        .set({
          status: 'idle',
          lastScanAt: new Date(),
        })
        .where(eq(disks.id, diskId));

      // Publish completion
      await redis.publish(`scan:${jobId}`, JSON.stringify({ type: 'complete' }));

    } catch (error) {
      // Handle errors
      await db.update(scanJobs)
        .set({
          status: 'failed',
          errorMessage: error instanceof Error ? error.message : 'Unknown error',
        })
        .where(eq(scanJobs.id, jobId));

      await db.update(disks)
        .set({ status: 'error' })
        .where(eq(disks.id, diskId));

      throw error;
    } finally {
      // Cleanup temp database
      await Bun.write(tempDbPath, '').catch(() => {});
    }
  },
  {
    connection: redis,
    concurrency: 2, // Max 2 concurrent scans
  }
);

async function importScanResults(
  sqlitePath: string,
  diskId: string,
  userId: string
) {
  // Use better-sqlite3 or similar to read results
  // and batch insert into PostgreSQL
  // ... implementation
}

worker.on('failed', (job, err) => {
  console.error(`Job ${job?.id} failed:`, err);
});

export { worker };
```

---

## File Storage & Thumbnails

### MinIO/S3 Configuration

```typescript
// src/lib/server/storage/s3.ts

import { S3Client, PutObjectCommand, GetObjectCommand } from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';

const s3 = new S3Client({
  endpoint: process.env.S3_ENDPOINT || 'http://localhost:9000',
  region: process.env.S3_REGION || 'us-east-1',
  credentials: {
    accessKeyId: process.env.S3_ACCESS_KEY || 'minioadmin',
    secretAccessKey: process.env.S3_SECRET_KEY || 'minioadmin',
  },
  forcePathStyle: true, // Required for MinIO
});

const BUCKET = process.env.S3_BUCKET || 'thumbnails';

export async function uploadThumbnail(
  key: string,
  data: Buffer,
  contentType: string = 'image/jpeg'
): Promise<string> {
  await s3.send(new PutObjectCommand({
    Bucket: BUCKET,
    Key: key,
    Body: data,
    ContentType: contentType,
  }));

  return key;
}

export async function getThumbnailUrl(key: string): Promise<string> {
  const command = new GetObjectCommand({
    Bucket: BUCKET,
    Key: key,
  });

  // Generate signed URL valid for 1 hour
  return getSignedUrl(s3, command, { expiresIn: 3600 });
}

export function generateThumbnailKey(userId: string, diskId: string, imageId: string): string {
  return `${userId}/${diskId}/${imageId}.jpg`;
}
```

### Thumbnail Generation Strategy

```typescript
// Generate thumbnails on-demand or during scan

// Option 1: On-demand (lazy) - Generate when first requested
// Pros: Saves storage, faster initial scan
// Cons: Slower first load for user

// Option 2: During scan - Generate as part of scan job
// Pros: Instant display
// Cons: Slower scans, more storage

// Recommended: Hybrid approach
// - Generate thumbnails in background queue after scan completes
// - Show placeholder/skeleton while generating
// - Cache in Redis for hot images
```

---

## Authentication & Authorization

### Lucia Auth Setup

```typescript
// src/lib/server/auth/lucia.ts

import { Lucia } from 'lucia';
import { DrizzlePostgreSQLAdapter } from '@lucia-auth/adapter-drizzle';
import { db } from '../db';
import { users, sessions } from '../db/schema';

const adapter = new DrizzlePostgreSQLAdapter(db, sessions, users);

export const lucia = new Lucia(adapter, {
  sessionCookie: {
    attributes: {
      secure: process.env.NODE_ENV === 'production',
    },
  },
  getUserAttributes: (attributes) => ({
    email: attributes.email,
    name: attributes.name,
    tier: attributes.tier,
  }),
});

declare module 'lucia' {
  interface Register {
    Lucia: typeof lucia;
    DatabaseUserAttributes: {
      email: string;
      name: string | null;
      tier: string;
    };
  }
}
```

### Auth Middleware

```typescript
// src/hooks.server.ts

import { lucia } from '$lib/server/auth/lucia';
import type { Handle } from '@sveltejs/kit';

export const handle: Handle = async ({ event, resolve }) => {
  const sessionId = event.cookies.get(lucia.sessionCookieName);

  if (!sessionId) {
    event.locals.user = null;
    event.locals.session = null;
    return resolve(event);
  }

  const { session, user } = await lucia.validateSession(sessionId);

  if (session?.fresh) {
    const sessionCookie = lucia.createSessionCookie(session.id);
    event.cookies.set(sessionCookie.name, sessionCookie.value, {
      path: '.',
      ...sessionCookie.attributes,
    });
  }

  if (!session) {
    const sessionCookie = lucia.createBlankSessionCookie();
    event.cookies.set(sessionCookie.name, sessionCookie.value, {
      path: '.',
      ...sessionCookie.attributes,
    });
  }

  event.locals.user = user;
  event.locals.session = session;

  // Protect dashboard routes
  if (event.url.pathname.startsWith('/dashboard') && !user) {
    return new Response(null, {
      status: 302,
      headers: { Location: '/auth/login' },
    });
  }

  return resolve(event);
};
```

---

## API Design

### Disk Management Endpoints

```typescript
// src/routes/api/disks/+server.ts

import { json } from '@sveltejs/kit';
import { db } from '$lib/server/db';
import { disks } from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { superValidate, message } from 'sveltekit-superforms/server';
import { diskSchema } from '$lib/schemas/disk';
import { zod } from 'sveltekit-superforms/adapters';

// GET /api/disks - List user's disks
export async function GET({ locals }) {
  if (!locals.user) {
    return json({ error: 'Unauthorized' }, { status: 401 });
  }

  const userDisks = await db.query.disks.findMany({
    where: eq(disks.userId, locals.user.id),
    orderBy: (disks, { desc }) => [desc(disks.updatedAt)],
  });

  return json({ disks: userDisks });
}

// POST /api/disks - Create new disk
export async function POST({ request, locals }) {
  if (!locals.user) {
    return json({ error: 'Unauthorized' }, { status: 401 });
  }

  const form = await superValidate(request, zod(diskSchema));

  if (!form.valid) {
    return json({ error: 'Validation failed', errors: form.errors }, { status: 400 });
  }

  const [newDisk] = await db.insert(disks)
    .values({
      userId: locals.user.id,
      name: form.data.name,
      description: form.data.description,
      mountPath: form.data.mountPath,
      scanPaths: form.data.scanPaths,
    })
    .returning();

  return json({ disk: newDisk }, { status: 201 });
}
```

### Scan Progress SSE Endpoint

```typescript
// src/routes/api/scan-progress/+server.ts

import { Redis } from 'ioredis';

const redis = new Redis(process.env.REDIS_URL || 'redis://localhost:6379');

export async function GET({ url, locals }) {
  if (!locals.user) {
    return new Response('Unauthorized', { status: 401 });
  }

  const jobId = url.searchParams.get('jobId');
  if (!jobId) {
    return new Response('Missing jobId', { status: 400 });
  }

  const stream = new ReadableStream({
    async start(controller) {
      const subscriber = redis.duplicate();
      await subscriber.subscribe(`scan:${jobId}`);

      subscriber.on('message', (channel, message) => {
        controller.enqueue(`data: ${message}\n\n`);

        const data = JSON.parse(message);
        if (data.type === 'complete' || data.type === 'error') {
          subscriber.unsubscribe();
          subscriber.quit();
          controller.close();
        }
      });
    },
  });

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
    },
  });
}
```

---

## Deployment Architecture

### Docker Compose Setup

```yaml
# docker-compose.yml

version: '3.8'

services:
  app:
    build: .
    ports:
      - "3000:3000"
    environment:
      - DATABASE_URL=postgresql://goimagefinder:password@postgres:5432/goimagefinder_db
      - REDIS_URL=redis://redis:6379
      - S3_ENDPOINT=http://minio:9000
      - S3_ACCESS_KEY=minioadmin
      - S3_SECRET_KEY=minioadmin
      - GOIMAGEFINDER_PATH=/app/bin/goimagefinder
    depends_on:
      - postgres
      - redis
      - minio
    volumes:
      - /mnt:/mnt:ro  # Mount external disks read-only
    restart: unless-stopped

  worker:
    build: .
    command: bun run workers/scan-worker.ts
    environment:
      - DATABASE_URL=postgresql://goimagefinder:password@postgres:5432/goimagefinder_db
      - REDIS_URL=redis://redis:6379
      - GOIMAGEFINDER_PATH=/app/bin/goimagefinder
    depends_on:
      - postgres
      - redis
    volumes:
      - /mnt:/mnt:ro
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=goimagefinder
      - POSTGRES_PASSWORD=password
      - POSTGRES_DB=goimagefinder_db
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
    volumes:
      - redis_data:/data
    restart: unless-stopped

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    volumes:
      - minio_data:/data
    ports:
      - "9001:9001"  # MinIO Console
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - app
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  minio_data:
```

### Nginx Configuration

```nginx
# nginx.conf

events {
    worker_connections 1024;
}

http {
    upstream app {
        server app:3000;
    }

    server {
        listen 80;
        server_name yourdomain.com;
        return 301 https://$server_name$request_uri;
    }

    server {
        listen 443 ssl http2;
        server_name yourdomain.com;

        ssl_certificate /etc/nginx/certs/fullchain.pem;
        ssl_certificate_key /etc/nginx/certs/privkey.pem;

        # Security headers
        add_header X-Frame-Options DENY;
        add_header X-Content-Type-Options nosniff;
        add_header X-XSS-Protection "1; mode=block";

        # Proxy to SvelteKit app
        location / {
            proxy_pass http://app;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection 'upgrade';
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_cache_bypass $http_upgrade;
        }

        # SSE endpoint - disable buffering
        location /api/scan-progress {
            proxy_pass http://app;
            proxy_http_version 1.1;
            proxy_set_header Connection '';
            proxy_buffering off;
            proxy_cache off;
            chunked_transfer_encoding off;
        }

        # Thumbnail proxy with caching
        location /api/images/ {
            proxy_pass http://app;
            proxy_cache_valid 200 1d;
            expires 1d;
            add_header Cache-Control "public, immutable";
        }
    }
}
```

---

## Reverse Proxy: Caddy vs Nginx

### Recommendation: Use Caddy

| Feature | Caddy | Nginx |
|---------|-------|-------|
| **Automatic HTTPS** | ✅ Built-in (Let's Encrypt) | ❌ Requires certbot + cron |
| **Configuration** | ✅ Simple Caddyfile | ❌ Complex syntax |
| **Hot reload** | ✅ Automatic | ❌ Manual `nginx -s reload` |
| **HTTP/3 (QUIC)** | ✅ Built-in | ❌ Requires compilation |
| **Memory usage** | Moderate (~20MB) | Lower (~5MB) |
| **Performance** | Excellent | Excellent |
| **Ecosystem** | Growing | Mature |

**Verdict:** For most SvelteKit apps, **Caddy wins** due to zero-config HTTPS and simpler maintenance. Nginx only makes sense if you need advanced load balancing or have existing Nginx expertise.

### Caddy Installation

```bash
# Ubuntu/Debian
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

### Caddyfile Configuration

```caddyfile
# /etc/caddy/Caddyfile

{
    email admin@yourdomain.com
}

yourdomain.com {
    # Automatic HTTPS with Let's Encrypt - no configuration needed!

    # Reverse proxy to SvelteKit
    reverse_proxy localhost:3000

    # SSE endpoint - disable buffering
    @sse path /api/scan-progress*
    reverse_proxy @sse localhost:3000 {
        flush_interval -1
        transport http {
            compression off
        }
    }

    # Thumbnail caching
    @images path /api/images/*
    header @images Cache-Control "public, max-age=86400, immutable"

    # Security headers
    header {
        X-Frame-Options DENY
        X-Content-Type-Options nosniff
        X-XSS-Protection "1; mode=block"
        Referrer-Policy strict-origin-when-cross-origin
    }

    # Gzip compression
    encode gzip

    # Logging
    log {
        output file /var/log/caddy/access.log
        format json
    }
}

# Redirect www to non-www
www.yourdomain.com {
    redir https://yourdomain.com{uri} permanent
}
```

### Docker Compose with Caddy

```yaml
# docker-compose.yml (Caddy version)

services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"  # HTTP/3
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - app
    restart: unless-stopped

volumes:
  caddy_data:    # Stores certificates
  caddy_config:  # Stores configuration
```

---

## Scalability & Horizontal Scaling

### Docker Compose Limitations

The basic Docker Compose setup works well for:
- **Up to ~1,000 concurrent users**
- **~100,000 images per user**
- **Single server deployment**

Beyond this, you'll hit bottlenecks in:
1. Database connections
2. Worker throughput
3. Memory for thumbnail generation
4. Disk I/O

### Scaling Strategy Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SCALING PROGRESSION                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│  PHASE 1: Single Server (Docker Compose)                                     │
│  ├── 1 App instance                                                          │
│  ├── 1 Worker instance                                                       │
│  ├── PostgreSQL on same server                                               │
│  └── Good for: MVP, < 1K users                                               │
│                                                                               │
│  PHASE 2: Vertical + Service Separation                                      │
│  ├── Bigger server (16 cores, 32GB RAM)                                      │
│  ├── 3 App instances behind Caddy                                            │
│  ├── 3 Worker instances                                                      │
│  ├── Managed PostgreSQL (RDS, Cloud SQL)                                     │
│  └── Good for: < 10K users                                                   │
│                                                                               │
│  PHASE 3: Multi-Server (Docker Swarm / Nomad)                                │
│  ├── 3+ App servers                                                          │
│  ├── Dedicated worker servers                                                │
│  ├── Redis Cluster                                                           │
│  ├── PostgreSQL with read replicas                                           │
│  └── Good for: < 100K users                                                  │
│                                                                               │
│  PHASE 4: Kubernetes                                                         │
│  ├── Auto-scaling pods                                                       │
│  ├── Horizontal Pod Autoscaler (HPA)                                         │
│  ├── Managed Kubernetes (EKS, GKE, AKS)                                      │
│  ├── Database connection pooling (PgBouncer)                                 │
│  └── Good for: 100K+ users                                                   │
│                                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Phase 2: Multi-Instance Docker Compose

```yaml
# docker-compose.scale.yml

services:
  app:
    build: .
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 2G
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis

  worker:
    build: .
    command: bun run workers/scan-worker.ts
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '4'
          memory: 4G
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - REDIS_URL=redis://redis:6379
    volumes:
      - /mnt:/mnt:ro

  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile.scale:/etc/caddy/Caddyfile:ro

  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
```

```caddyfile
# Caddyfile.scale - Load balancing multiple app instances

yourdomain.com {
    reverse_proxy app:3000 {
        lb_policy round_robin
        health_uri /api/health
        health_interval 10s
    }
}
```

### Phase 4: Kubernetes Deployment

```yaml
# k8s/deployment.yaml

apiVersion: apps/v1
kind: Deployment
metadata:
  name: imagefinder-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: imagefinder
  template:
    metadata:
      labels:
        app: imagefinder
    spec:
      containers:
        - name: app
          image: yourregistry/imagefinder:latest
          ports:
            - containerPort: 3000
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "2000m"
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: url
          readinessProbe:
            httpGet:
              path: /api/health
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 10

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: imagefinder-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: imagefinder-app
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### Database Scaling Strategies

```
┌────────────────────────────────────────────────────────────────┐
│                    DATABASE SCALING                             │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Connection Pooling (PgBouncer)                             │
│     - Pool connections at 100-200 max                          │
│     - Prevents connection exhaustion                           │
│     - Add between app and database                             │
│                                                                 │
│  2. Read Replicas                                               │
│     - Route SELECT queries to replicas                         │
│     - Keep writes on primary                                   │
│     - Use Drizzle's replica support                            │
│                                                                 │
│  3. Table Partitioning                                          │
│     - Partition images table by user_id                        │
│     - Faster queries, easier maintenance                       │
│     - PostgreSQL native partitioning                           │
│                                                                 │
│  4. Caching Layer                                               │
│     - Redis for hot queries                                    │
│     - Cache user's disk/image counts                           │
│     - TTL-based invalidation                                   │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### Managed Services Recommendations

| Service | Self-Hosted | Cloud Managed |
|---------|-------------|---------------|
| PostgreSQL | Docker container | AWS RDS, Supabase, Neon |
| Redis | Docker container | AWS ElastiCache, Upstash |
| Object Storage | MinIO | AWS S3, Cloudflare R2 |
| Container Orchestration | Docker Swarm | AWS ECS, Fly.io, Railway |
| Kubernetes | k3s | AWS EKS, GKE, DigitalOcean K8s |

---

## Subscription & Payment System

### Pricing Tiers

| Tier | Price | Features |
|------|-------|----------|
| **Free** | $0/mo | 2 disks, 5,000 images, basic search |
| **Pro** | $9/mo or $90/yr | 10 disks, 100,000 images, batch search, priority support |
| **Team** | $29/mo or $290/yr | Unlimited disks, 500,000 images, 5 team members, API access |
| **Enterprise** | Custom | Unlimited everything, dedicated support, SLA |

### Database Schema Additions

```typescript
// src/lib/server/db/schema.ts (additions)

// ============================================
// SUBSCRIPTION PLANS TABLE
// ============================================
export const plans = pgTable('plans', {
  id: text('id').primaryKey(), // 'free', 'pro', 'team', 'enterprise'
  name: text('name').notNull(),
  description: text('description'),

  // Pricing
  priceMonthly: integer('price_monthly'),      // Cents (e.g., 900 = $9.00)
  priceYearly: integer('price_yearly'),        // Cents (e.g., 9000 = $90.00)
  stripePriceIdMonthly: text('stripe_price_id_monthly'),
  stripePriceIdYearly: text('stripe_price_id_yearly'),

  // Limits
  maxDisks: integer('max_disks').notNull(),
  maxImagesPerDisk: integer('max_images_per_disk').notNull(),
  maxTotalImages: integer('max_total_images').notNull(),
  maxTeamMembers: integer('max_team_members').default(1),

  // Features (stored as JSON or separate columns)
  featureBatchSearch: boolean('feature_batch_search').default(false),
  featureApiAccess: boolean('feature_api_access').default(false),
  featurePrioritySupport: boolean('feature_priority_support').default(false),
  featureScheduledScans: boolean('feature_scheduled_scans').default(false),

  createdAt: timestamp('created_at').defaultNow().notNull(),
});

// ============================================
// SUBSCRIPTIONS TABLE
// ============================================
export const subscriptions = pgTable('subscriptions', {
  id: uuid('id').defaultRandom().primaryKey(),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
  planId: text('plan_id').notNull().references(() => plans.id),

  // Stripe data
  stripeCustomerId: text('stripe_customer_id'),
  stripeSubscriptionId: text('stripe_subscription_id'),
  stripePriceId: text('stripe_price_id'),

  // Status
  status: text('status').notNull().default('active'), // 'active', 'canceled', 'past_due', 'trialing'
  billingCycle: text('billing_cycle').default('monthly'), // 'monthly', 'yearly'

  // Dates
  currentPeriodStart: timestamp('current_period_start'),
  currentPeriodEnd: timestamp('current_period_end'),
  canceledAt: timestamp('canceled_at'),
  trialEndsAt: timestamp('trial_ends_at'),

  createdAt: timestamp('created_at').defaultNow().notNull(),
  updatedAt: timestamp('updated_at').defaultNow().notNull(),
}, (table) => ({
  userIdx: index('subscriptions_user_idx').on(table.userId),
  stripeSubIdx: uniqueIndex('subscriptions_stripe_sub_idx').on(table.stripeSubscriptionId),
}));

// ============================================
// INVOICES TABLE (for history)
// ============================================
export const invoices = pgTable('invoices', {
  id: uuid('id').defaultRandom().primaryKey(),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),
  subscriptionId: uuid('subscription_id').references(() => subscriptions.id),

  stripeInvoiceId: text('stripe_invoice_id').unique(),
  stripePaymentIntentId: text('stripe_payment_intent_id'),

  amount: integer('amount').notNull(),        // Cents
  currency: text('currency').default('usd'),
  status: text('status').notNull(),           // 'paid', 'open', 'void', 'uncollectible'

  invoiceUrl: text('invoice_url'),            // Stripe hosted invoice URL
  invoicePdf: text('invoice_pdf'),            // PDF download URL

  paidAt: timestamp('paid_at'),
  createdAt: timestamp('created_at').defaultNow().notNull(),
});

// ============================================
// USAGE TRACKING TABLE (for metered billing or limits)
// ============================================
export const usageRecords = pgTable('usage_records', {
  id: uuid('id').defaultRandom().primaryKey(),
  userId: uuid('user_id').notNull().references(() => users.id, { onDelete: 'cascade' }),

  // Usage metrics
  totalImages: integer('total_images').default(0),
  totalDisks: integer('total_disks').default(0),
  totalStorageBytes: integer('total_storage_bytes').default(0),
  searchesThisMonth: integer('searches_this_month').default(0),
  scansThisMonth: integer('scans_this_month').default(0),

  // Reset tracking
  periodStart: timestamp('period_start').notNull(),
  periodEnd: timestamp('period_end').notNull(),

  updatedAt: timestamp('updated_at').defaultNow().notNull(),
}, (table) => ({
  userPeriodIdx: uniqueIndex('usage_user_period_idx').on(table.userId, table.periodStart),
}));
```

### Stripe Integration

```typescript
// src/lib/server/payments/stripe.ts

import Stripe from 'stripe';

export const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!, {
  apiVersion: '2024-12-18.acacia',
});

// Create a checkout session for subscription
export async function createCheckoutSession(
  userId: string,
  email: string,
  priceId: string,
  successUrl: string,
  cancelUrl: string
): Promise<string> {
  const session = await stripe.checkout.sessions.create({
    customer_email: email,
    mode: 'subscription',
    payment_method_types: ['card'],
    line_items: [
      {
        price: priceId,
        quantity: 1,
      },
    ],
    success_url: `${successUrl}?session_id={CHECKOUT_SESSION_ID}`,
    cancel_url: cancelUrl,
    metadata: {
      userId,
    },
    subscription_data: {
      metadata: {
        userId,
      },
    },
    allow_promotion_codes: true,
  });

  return session.url!;
}

// Create customer portal session for managing subscription
export async function createPortalSession(
  customerId: string,
  returnUrl: string
): Promise<string> {
  const session = await stripe.billingPortal.sessions.create({
    customer: customerId,
    return_url: returnUrl,
  });

  return session.url;
}

// Get subscription details
export async function getSubscription(subscriptionId: string) {
  return stripe.subscriptions.retrieve(subscriptionId, {
    expand: ['default_payment_method', 'latest_invoice'],
  });
}

// Cancel subscription at period end
export async function cancelSubscription(subscriptionId: string) {
  return stripe.subscriptions.update(subscriptionId, {
    cancel_at_period_end: true,
  });
}

// Resume canceled subscription
export async function resumeSubscription(subscriptionId: string) {
  return stripe.subscriptions.update(subscriptionId, {
    cancel_at_period_end: false,
  });
}
```

### Stripe Webhook Handler

```typescript
// src/routes/api/webhooks/stripe/+server.ts

import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { stripe } from '$lib/server/payments/stripe';
import { db } from '$lib/server/db';
import { subscriptions, users, invoices } from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';

const webhookSecret = process.env.STRIPE_WEBHOOK_SECRET!;

export const POST: RequestHandler = async ({ request }) => {
  const body = await request.text();
  const signature = request.headers.get('stripe-signature')!;

  let event: Stripe.Event;

  try {
    event = stripe.webhooks.constructEvent(body, signature, webhookSecret);
  } catch (err) {
    console.error('Webhook signature verification failed:', err);
    return json({ error: 'Invalid signature' }, { status: 400 });
  }

  switch (event.type) {
    case 'checkout.session.completed': {
      const session = event.data.object as Stripe.Checkout.Session;
      await handleCheckoutComplete(session);
      break;
    }

    case 'customer.subscription.updated': {
      const subscription = event.data.object as Stripe.Subscription;
      await handleSubscriptionUpdate(subscription);
      break;
    }

    case 'customer.subscription.deleted': {
      const subscription = event.data.object as Stripe.Subscription;
      await handleSubscriptionCanceled(subscription);
      break;
    }

    case 'invoice.paid': {
      const invoice = event.data.object as Stripe.Invoice;
      await handleInvoicePaid(invoice);
      break;
    }

    case 'invoice.payment_failed': {
      const invoice = event.data.object as Stripe.Invoice;
      await handlePaymentFailed(invoice);
      break;
    }
  }

  return json({ received: true });
};

async function handleCheckoutComplete(session: Stripe.Checkout.Session) {
  const userId = session.metadata?.userId;
  if (!userId) return;

  const subscriptionId = session.subscription as string;
  const subscription = await stripe.subscriptions.retrieve(subscriptionId);

  // Determine plan from price
  const priceId = subscription.items.data[0].price.id;
  const planId = await getPlanIdFromPrice(priceId);

  // Create or update subscription record
  await db.insert(subscriptions)
    .values({
      userId,
      planId,
      stripeCustomerId: session.customer as string,
      stripeSubscriptionId: subscriptionId,
      stripePriceId: priceId,
      status: subscription.status,
      billingCycle: subscription.items.data[0].price.recurring?.interval === 'year' ? 'yearly' : 'monthly',
      currentPeriodStart: new Date(subscription.current_period_start * 1000),
      currentPeriodEnd: new Date(subscription.current_period_end * 1000),
    })
    .onConflictDoUpdate({
      target: subscriptions.stripeSubscriptionId,
      set: {
        status: subscription.status,
        currentPeriodEnd: new Date(subscription.current_period_end * 1000),
        updatedAt: new Date(),
      },
    });

  // Update user tier
  await db.update(users)
    .set({ tier: planId })
    .where(eq(users.id, userId));
}

async function handleSubscriptionUpdate(subscription: Stripe.Subscription) {
  await db.update(subscriptions)
    .set({
      status: subscription.status,
      currentPeriodStart: new Date(subscription.current_period_start * 1000),
      currentPeriodEnd: new Date(subscription.current_period_end * 1000),
      canceledAt: subscription.canceled_at ? new Date(subscription.canceled_at * 1000) : null,
      updatedAt: new Date(),
    })
    .where(eq(subscriptions.stripeSubscriptionId, subscription.id));
}

async function handleSubscriptionCanceled(subscription: Stripe.Subscription) {
  const userId = subscription.metadata?.userId;

  await db.update(subscriptions)
    .set({
      status: 'canceled',
      canceledAt: new Date(),
      updatedAt: new Date(),
    })
    .where(eq(subscriptions.stripeSubscriptionId, subscription.id));

  // Downgrade user to free tier
  if (userId) {
    await db.update(users)
      .set({ tier: 'free' })
      .where(eq(users.id, userId));
  }
}

async function handleInvoicePaid(invoice: Stripe.Invoice) {
  const userId = invoice.subscription_details?.metadata?.userId;
  if (!userId) return;

  await db.insert(invoices).values({
    userId,
    stripeInvoiceId: invoice.id,
    stripePaymentIntentId: invoice.payment_intent as string,
    amount: invoice.amount_paid,
    currency: invoice.currency,
    status: 'paid',
    invoiceUrl: invoice.hosted_invoice_url,
    invoicePdf: invoice.invoice_pdf,
    paidAt: new Date(),
  });
}

async function handlePaymentFailed(invoice: Stripe.Invoice) {
  // Update subscription status
  if (invoice.subscription) {
    await db.update(subscriptions)
      .set({ status: 'past_due', updatedAt: new Date() })
      .where(eq(subscriptions.stripeSubscriptionId, invoice.subscription as string));
  }

  // TODO: Send email notification to user
}

async function getPlanIdFromPrice(priceId: string): Promise<string> {
  const plan = await db.query.plans.findFirst({
    where: (plans, { or, eq }) => or(
      eq(plans.stripePriceIdMonthly, priceId),
      eq(plans.stripePriceIdYearly, priceId)
    ),
  });
  return plan?.id ?? 'free';
}
```

### Pricing Page Component

```svelte
<!-- src/routes/pricing/+page.svelte -->

<script lang="ts">
  import { enhance } from '$app/forms';

  export let data;

  let billingCycle: 'monthly' | 'yearly' = 'yearly';

  const plans = [
    {
      id: 'free',
      name: 'Free',
      description: 'For personal use',
      priceMonthly: 0,
      priceYearly: 0,
      features: [
        '2 disks',
        '5,000 images',
        'Basic search',
        'Community support',
      ],
      cta: 'Get Started',
    },
    {
      id: 'pro',
      name: 'Pro',
      description: 'For power users',
      priceMonthly: 9,
      priceYearly: 90,
      features: [
        '10 disks',
        '100,000 images',
        'Batch search',
        'Priority support',
        'Scheduled scans',
      ],
      cta: 'Start Free Trial',
      popular: true,
    },
    {
      id: 'team',
      name: 'Team',
      description: 'For small teams',
      priceMonthly: 29,
      priceYearly: 290,
      features: [
        'Unlimited disks',
        '500,000 images',
        '5 team members',
        'API access',
        'Advanced analytics',
        'Dedicated support',
      ],
      cta: 'Start Free Trial',
    },
  ];

  function getPrice(plan: typeof plans[0]) {
    if (plan.priceMonthly === 0) return 'Free';
    const price = billingCycle === 'yearly'
      ? Math.floor(plan.priceYearly / 12)
      : plan.priceMonthly;
    return `$${price}`;
  }

  function getSavings(plan: typeof plans[0]) {
    if (plan.priceMonthly === 0) return null;
    const monthly = plan.priceMonthly * 12;
    const yearly = plan.priceYearly;
    const savings = Math.round((1 - yearly / monthly) * 100);
    return savings > 0 ? `Save ${savings}%` : null;
  }
</script>

<div class="max-w-6xl mx-auto px-4 py-16">
  <div class="text-center mb-12">
    <h1 class="text-4xl font-bold mb-4">Simple, transparent pricing</h1>
    <p class="text-gray-600 mb-8">Start free, upgrade when you need more</p>

    <!-- Billing toggle -->
    <div class="inline-flex items-center gap-4 bg-gray-100 rounded-full p-1">
      <button
        class="px-4 py-2 rounded-full transition-colors"
        class:bg-white={billingCycle === 'monthly'}
        class:shadow={billingCycle === 'monthly'}
        onclick={() => billingCycle = 'monthly'}
      >
        Monthly
      </button>
      <button
        class="px-4 py-2 rounded-full transition-colors"
        class:bg-white={billingCycle === 'yearly'}
        class:shadow={billingCycle === 'yearly'}
        onclick={() => billingCycle = 'yearly'}
      >
        Yearly
        <span class="text-green-600 text-sm ml-1">Save 17%</span>
      </button>
    </div>
  </div>

  <div class="grid md:grid-cols-3 gap-8">
    {#each plans as plan}
      <div
        class="border rounded-2xl p-8 relative"
        class:border-blue-500={plan.popular}
        class:border-2={plan.popular}
      >
        {#if plan.popular}
          <div class="absolute -top-3 left-1/2 -translate-x-1/2 bg-blue-500 text-white px-3 py-1 rounded-full text-sm">
            Most Popular
          </div>
        {/if}

        <h3 class="text-xl font-semibold">{plan.name}</h3>
        <p class="text-gray-600 text-sm mb-4">{plan.description}</p>

        <div class="mb-6">
          <span class="text-4xl font-bold">{getPrice(plan)}</span>
          {#if plan.priceMonthly > 0}
            <span class="text-gray-600">/month</span>
          {/if}
          {#if billingCycle === 'yearly' && getSavings(plan)}
            <span class="block text-green-600 text-sm">{getSavings(plan)}</span>
          {/if}
        </div>

        <form method="POST" action="?/subscribe" use:enhance>
          <input type="hidden" name="planId" value={plan.id} />
          <input type="hidden" name="billingCycle" value={billingCycle} />
          <button
            type="submit"
            class="w-full py-3 rounded-lg font-medium transition-colors"
            class:bg-blue-600={plan.popular}
            class:text-white={plan.popular}
            class:hover:bg-blue-700={plan.popular}
            class:bg-gray-100={!plan.popular}
            class:hover:bg-gray-200={!plan.popular}
            disabled={data.user?.tier === plan.id}
          >
            {data.user?.tier === plan.id ? 'Current Plan' : plan.cta}
          </button>
        </form>

        <ul class="mt-8 space-y-3">
          {#each plan.features as feature}
            <li class="flex items-center gap-2">
              <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
              {feature}
            </li>
          {/each}
        </ul>
      </div>
    {/each}
  </div>
</div>
```

### Usage Enforcement Middleware

```typescript
// src/lib/server/middleware/usage.ts

import { db } from '$lib/server/db';
import { users, disks, images, plans } from '$lib/server/db/schema';
import { eq, count } from 'drizzle-orm';

interface UsageLimits {
  maxDisks: number;
  maxImagesPerDisk: number;
  maxTotalImages: number;
  canBatchSearch: boolean;
  canUseApi: boolean;
}

export async function getUserLimits(userId: string): Promise<UsageLimits> {
  const user = await db.query.users.findFirst({
    where: eq(users.id, userId),
  });

  const plan = await db.query.plans.findFirst({
    where: eq(plans.id, user?.tier ?? 'free'),
  });

  return {
    maxDisks: plan?.maxDisks ?? 2,
    maxImagesPerDisk: plan?.maxImagesPerDisk ?? 5000,
    maxTotalImages: plan?.maxTotalImages ?? 5000,
    canBatchSearch: plan?.featureBatchSearch ?? false,
    canUseApi: plan?.featureApiAccess ?? false,
  };
}

export async function checkCanCreateDisk(userId: string): Promise<{ allowed: boolean; reason?: string }> {
  const limits = await getUserLimits(userId);

  const [{ count: diskCount }] = await db
    .select({ count: count() })
    .from(disks)
    .where(eq(disks.userId, userId));

  if (diskCount >= limits.maxDisks) {
    return {
      allowed: false,
      reason: `You've reached your limit of ${limits.maxDisks} disks. Upgrade to add more.`,
    };
  }

  return { allowed: true };
}

export async function checkCanScanImages(
  userId: string,
  diskId: string,
  newImageCount: number
): Promise<{ allowed: boolean; reason?: string }> {
  const limits = await getUserLimits(userId);

  // Check per-disk limit
  const [{ count: diskImageCount }] = await db
    .select({ count: count() })
    .from(images)
    .where(eq(images.diskId, diskId));

  if (diskImageCount + newImageCount > limits.maxImagesPerDisk) {
    return {
      allowed: false,
      reason: `This disk would exceed the ${limits.maxImagesPerDisk.toLocaleString()} images limit.`,
    };
  }

  // Check total limit
  const [{ count: totalImageCount }] = await db
    .select({ count: count() })
    .from(images)
    .where(eq(images.userId, userId));

  if (totalImageCount + newImageCount > limits.maxTotalImages) {
    return {
      allowed: false,
      reason: `You would exceed your total limit of ${limits.maxTotalImages.toLocaleString()} images.`,
    };
  }

  return { allowed: true };
}
```

### Environment Variables for Stripe

```bash
# .env additions for Stripe

# Stripe
STRIPE_SECRET_KEY=sk_live_...
STRIPE_PUBLISHABLE_KEY=pk_live_...
STRIPE_WEBHOOK_SECRET=whsec_...

# Price IDs (create these in Stripe Dashboard)
STRIPE_PRICE_PRO_MONTHLY=price_...
STRIPE_PRICE_PRO_YEARLY=price_...
STRIPE_PRICE_TEAM_MONTHLY=price_...
STRIPE_PRICE_TEAM_YEARLY=price_...
```

---

## Expanded Vision & Future Features

### Phase 1: Core MVP
- [x] User registration/login
- [x] Disk management (CRUD)
- [x] Scan disks using Go CLI
- [x] View images with thumbnails
- [x] Basic similarity search

### Phase 2: Enhanced Search
- [ ] **Batch search** - Upload multiple query images
- [ ] **Cross-disk search** - Find duplicates across all user disks
- [ ] **Smart albums** - Auto-group similar images
- [ ] **Face detection** - Group photos by person (optional OpenCV integration)
- [ ] **EXIF filtering** - Search by date, camera, location

### Phase 3: Collaboration
- [ ] **Shared disks** - Multiple users can view same disk
- [ ] **Teams/Organizations** - Group accounts with shared storage
- [ ] **Comments/Tags** - Annotate images
- [ ] **Export collections** - Download matched images as ZIP

### Phase 4: Intelligence
- [ ] **AI tagging** - Auto-tag images using vision models
- [ ] **Natural language search** - "Find beach photos from 2023"
- [ ] **Duplicate cleanup** - Suggest which duplicates to keep/delete
- [ ] **Storage analytics** - Disk usage reports, format distribution

### Phase 5: Scale & Performance
- [ ] **Distributed scanning** - Multiple workers on different machines
- [ ] **Pre-computed similarity matrix** - Instant duplicate detection
- [ ] **CDN integration** - CloudFlare/AWS CloudFront for thumbnails
- [ ] **Mobile app** - React Native companion app

### Feature Ideas to Consider

1. **Remote disk scanning via agent**
   - Install lightweight Go agent on remote machines
   - Agent pushes scan results to cloud
   - Useful for NAS devices, home servers

2. **Scheduled scans**
   - Cron-like scheduling for automatic re-scans
   - Detect new/deleted images

3. **Webhook notifications**
   - Notify external systems when scans complete
   - Integration with Slack, Discord, email

4. **API access**
   - RESTful API for third-party integrations
   - API key management

5. **Import from cloud services**
   - Google Photos, Dropbox, iCloud
   - Compare cloud photos with local disks

---

## Environment Variables Reference

```bash
# .env.example

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/goimagefinder_db

# Redis
REDIS_URL=redis://localhost:6379

# S3/MinIO
S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=thumbnails

# Go CLI
GOIMAGEFINDER_PATH=/usr/local/bin/goimagefinder

# Auth
AUTH_SECRET=your-super-secret-key-min-32-chars

# App
PUBLIC_APP_URL=https://yourdomain.com
NODE_ENV=production
```

---

## Summary

This architecture provides:

1. **Multi-tenancy**: Complete user isolation with proper authorization
2. **Scalability**: Stateless app servers + Redis queues + PostgreSQL
3. **Performance**: Go CLI for heavy image processing, thumbnails cached in S3
4. **Real-time updates**: SSE for scan progress via Redis Pub/Sub
5. **Security**: HTTPS, session management, rate limiting
6. **Extensibility**: Clean separation of concerns, ready for future features

The hybrid PostgreSQL + Redis approach gives you the best of both worlds: relational integrity for core data and blazing-fast caching/queuing for real-time operations.
