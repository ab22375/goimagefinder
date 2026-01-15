let selectedImageFiles = [];
const MAX_IMAGES = 20;
let config = {};

function updateThreshold() {
    const slider = document.getElementById('threshold');
    const value = slider.value / 100;
    document.getElementById('thresholdValue').textContent = value.toFixed(2);
}

let browseModal = null;
let browseCallback = null;
let currentBrowseType = 'folder';

function browsePath(type) {
    currentBrowseType = (type === 'db') ? 'file' : 'folder';
    const currentPath = (type === 'db') ? 
        document.getElementById('dbPath').value : 
        document.getElementById('folderPath').value;
    
    browseCallback = (selectedPath) => {
        if (type === 'db') {
            document.getElementById('dbPath').value = selectedPath;
            updateDatabaseInfo();
        } else {
            document.getElementById('folderPath').value = selectedPath;
        }
        saveConfig();
    };
    
    showBrowseModal(currentPath || '');
}

function showBrowseModal(initialPath) {
    // Create modal if it doesn't exist
    if (!browseModal) {
        browseModal = document.createElement('div');
        browseModal.className = 'browse-modal';
        browseModal.innerHTML = `
            <div class="browse-content">
                <div class="browse-header">
                    <h3>Select ${currentBrowseType === 'file' ? 'Database File' : 'Folder'}</h3>
                    <button class="browse-close" onclick="closeBrowseModal()">×</button>
                </div>
                <div class="browse-path">
                    <input type="text" id="browsePath" readonly>
                    <button onclick="navigateUp()">↑ Parent</button>
                </div>
                <div class="browse-list" id="browseList">
                    Loading...
                </div>
                <div class="browse-footer">
                    <button class="browse-cancel" onclick="closeBrowseModal()">Cancel</button>
                    <button class="browse-select" onclick="selectPath()">Select</button>
                </div>
            </div>
        `;
        document.body.appendChild(browseModal);
    }
    
    browseModal.style.display = 'flex';
    loadDirectory(initialPath);
}

function closeBrowseModal() {
    if (browseModal) {
        browseModal.style.display = 'none';
    }
}

async function loadDirectory(path) {
    const browseList = document.getElementById('browseList');
    const browsePath = document.getElementById('browsePath');
    
    try {
        const response = await fetch(`/api/browse?path=${encodeURIComponent(path)}&type=${currentBrowseType}`);
        const data = await response.json();
        
        browsePath.value = data.currentPath;
        browseList.innerHTML = '';
        
        // Sort entries: directories first, then by name
        data.entries.sort((a, b) => {
            if (a.isDir !== b.isDir) return b.isDir - a.isDir;
            return a.name.localeCompare(b.name);
        });
        
        data.entries.forEach(entry => {
            const item = document.createElement('div');
            item.className = 'browse-item';
            if (entry.isDir) item.classList.add('browse-folder');
            
            item.innerHTML = `
                <span class="browse-icon">${entry.isDir ? '📁' : '📄'}</span>
                <span class="browse-name">${entry.name}</span>
                <span class="browse-size">${entry.isDir ? '' : formatSize(entry.size)}</span>
                <span class="browse-date">${entry.modified}</span>
            `;
            
            item.onclick = () => {
                if (entry.isDir) {
                    loadDirectory(entry.path);
                } else if (currentBrowseType === 'file') {
                    document.getElementById('browsePath').value = entry.path;
                }
            };
            
            browseList.appendChild(item);
        });
        
        if (data.entries.length === 0) {
            browseList.innerHTML = '<div class="browse-empty">No items to display</div>';
        }
    } catch (error) {
        browseList.innerHTML = '<div class="browse-error">Error loading directory</div>';
    }
}

function navigateUp() {
    const currentPath = document.getElementById('browsePath').value;
    const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
    loadDirectory(parentPath);
}

function selectPath() {
    const selectedPath = document.getElementById('browsePath').value;
    if (browseCallback) {
        browseCallback(selectedPath);
    }
    closeBrowseModal();
}

function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 10) / 10 + ' ' + sizes[i];
}

function handleFileSelect(event) {
    const files = Array.from(event.target.files);
    if (files.length === 0) return;

    // Check limit
    const remainingSlots = MAX_IMAGES - selectedImageFiles.length;
    const filesToAdd = files.slice(0, remainingSlots);

    if (files.length > remainingSlots) {
        showError(`Only ${remainingSlots} more images can be added (max ${MAX_IMAGES})`);
    }

    // Add files to selection
    filesToAdd.forEach(file => {
        selectedImageFiles.push(file);
    });

    updateSelectedImagesUI();
    updateSearchButton();

    // Reset file input so same file can be selected again
    event.target.value = '';
}

function updateSelectedImagesUI() {
    const grid = document.getElementById('selectedImages');
    const info = document.getElementById('selectedImagesInfo');
    const clearBtn = document.getElementById('clearBtn');

    grid.innerHTML = '';

    if (selectedImageFiles.length === 0) {
        info.textContent = '';
        clearBtn.style.display = 'none';
        return;
    }

    info.textContent = `${selectedImageFiles.length} image${selectedImageFiles.length > 1 ? 's' : ''} selected`;
    clearBtn.style.display = 'block';

    selectedImageFiles.forEach((file, index) => {
        const item = document.createElement('div');
        item.className = 'selected-image-item';

        const img = document.createElement('img');
        const reader = new FileReader();
        reader.onload = function(e) {
            img.src = e.target.result;
        };
        reader.readAsDataURL(file);

        const removeBtn = document.createElement('button');
        removeBtn.className = 'remove-btn';
        removeBtn.innerHTML = '&times;';
        removeBtn.onclick = (e) => {
            e.stopPropagation();
            removeSelectedImage(index);
        };

        const nameLabel = document.createElement('div');
        nameLabel.className = 'image-name';
        nameLabel.textContent = file.name;

        item.appendChild(img);
        item.appendChild(removeBtn);
        item.appendChild(nameLabel);
        grid.appendChild(item);
    });
}

function removeSelectedImage(index) {
    selectedImageFiles.splice(index, 1);
    updateSelectedImagesUI();
    updateSearchButton();
}

function clearSelectedImages() {
    selectedImageFiles = [];
    updateSelectedImagesUI();
    updateSearchButton();
}

function updateSearchButton() {
    const btn = document.getElementById('searchBtn');
    const count = selectedImageFiles.length;

    if (count === 0) {
        btn.textContent = 'Search';
    } else if (count === 1) {
        btn.textContent = 'Search';
    } else {
        btn.textContent = `Search ${count} Images`;
    }
}

async function startScan() {
    const dbPath = document.getElementById('dbPath').value;
    const folderPath = document.getElementById('folderPath').value;
    const prefix = document.getElementById('prefix').value;
    const forceRewrite = document.getElementById('forceRewrite').checked;

    if (!dbPath || !folderPath) {
        alert('Please provide both database path and folder path');
        return;
    }
    
    // Save config
    saveConfig();

    const progressBar = document.getElementById('progressBar');
    const progressFill = document.querySelector('.progress-fill');
    const progressText = document.querySelector('.progress-text');
    progressBar.style.display = 'block';
    
    // Reset progress
    progressFill.style.width = '0%';
    progressText.textContent = 'Starting scan...';

    try {
        const response = await fetch('/api/scan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                databasePath: dbPath,
                folderPath: folderPath,
                prefix: prefix,
                forceRewrite: forceRewrite
            })
        });

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let totalImages = 0;

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const chunk = decoder.decode(value);
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.substring(6));
                        
                        if (data.error) {
                            showError('Scan error: ' + data.error);
                            progressBar.style.display = 'none';
                        } else if (data.complete) {
                            progressFill.style.width = '100%';
                            progressText.textContent = `Scan completed! ${data.total || totalImages} images indexed.`;
                            showSuccess('Scan completed successfully!');
                            setTimeout(() => {
                                progressBar.style.display = 'none';
                            }, 3000);
                        } else if (data.total) {
                            totalImages = data.total;
                            progressText.textContent = data.message || `Found ${totalImages} images`;
                        } else if (data.current) {
                            const percentage = totalImages > 0 ? (data.current / totalImages * 100) : 0;
                            progressFill.style.width = percentage + '%';
                            progressText.textContent = `${data.current}/${totalImages} - ${data.message || 'Processing...'}`;
                        } else if (data.status === 'scanning') {
                            // Heartbeat - update text to show activity
                            if (!progressText.textContent.includes('...')) {
                                progressText.textContent += '.';
                            }
                        }
                    } catch (e) {
                        console.error('Error parsing SSE data:', e);
                    }
                }
            }
        }
    } catch (error) {
        showError('Error during scan: ' + error.message);
        progressBar.style.display = 'none';
    }
}

async function searchImages() {
    const dbPath = document.getElementById('dbPath').value;
    const threshold = parseFloat(document.getElementById('thresholdValue').textContent);

    if (!dbPath) {
        alert('Please provide database path');
        return;
    }

    if (selectedImageFiles.length === 0) {
        alert('Please select at least one image to search');
        return;
    }

    // Save config
    saveConfig();

    // Show loading
    const resultsContainer = document.getElementById('results');
    const searchBtn = document.getElementById('searchBtn');
    resultsContainer.innerHTML = '<p style="text-align: center; color: #666;">Searching...</p>';
    searchBtn.disabled = true;

    // Use batch search for multiple images, single search for one
    if (selectedImageFiles.length === 1) {
        // Single image search (legacy endpoint)
        const formData = new FormData();
        formData.append('image', selectedImageFiles[0]);
        formData.append('databasePath', dbPath);
        formData.append('threshold', threshold);

        try {
            const response = await fetch('/api/upload-search', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                throw new Error(await response.text());
            }

            const results = await response.json();
            displayResults(results);
        } catch (error) {
            showError('Error during search: ' + error.message);
            resultsContainer.innerHTML = '<p style="text-align: center; color: #f00;">Search failed</p>';
        } finally {
            searchBtn.disabled = false;
        }
    } else {
        // Batch search for multiple images
        const formData = new FormData();
        formData.append('databasePath', dbPath);
        formData.append('threshold', threshold);

        selectedImageFiles.forEach(file => {
            formData.append('images', file);
        });

        try {
            const response = await fetch('/api/batch-search', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                throw new Error(await response.text());
            }

            const results = await response.json();
            displayBatchResults(results);
        } catch (error) {
            showError('Error during batch search: ' + error.message);
            resultsContainer.innerHTML = '<p style="text-align: center; color: #f00;">Search failed</p>';
        } finally {
            searchBtn.disabled = false;
        }
    }
}

function displayResults(results) {
    const resultsContainer = document.getElementById('results');
    resultsContainer.innerHTML = '';

    if (results.length === 0) {
        resultsContainer.innerHTML = '<p style="text-align: center; color: #666;">No similar images found</p>';
        return;
    }

    results.forEach(result => {
        const resultItem = document.createElement('div');
        resultItem.className = 'result-item';
        
        // Use thumbnail parameter for better performance
        const thumbnailUrl = `/api/file?path=${encodeURIComponent(result.path)}&thumbnail=true`;
        
        resultItem.innerHTML = `
            <div class="result-preview" onclick="openFile('${result.path.replace(/'/g, "\\'")}')">
                <img src="${thumbnailUrl}" 
                     alt="Result" 
                     onerror="this.style.display='none'; this.parentElement.innerHTML='<span style=\\"color:#999;font-size:12px\\">No preview</span>'">
            </div>
            <div class="result-path" title="${result.path}" onclick="openFile('${result.path.replace(/'/g, "\\'")}')">${result.path}</div>
            <div class="result-score">${result.score.toFixed(2)}</div>
            <div class="result-actions">
                <button class="action-btn copy" onclick="copyPath('${result.path.replace(/'/g, "\\'")}', this)">Copy</button>
            </div>
        `;
        
        resultsContainer.appendChild(resultItem);
    });
}

function displayBatchResults(batchResults) {
    const resultsContainer = document.getElementById('results');
    resultsContainer.innerHTML = '';

    if (!batchResults || batchResults.length === 0) {
        resultsContainer.innerHTML = '<p style="text-align: center; color: #666;">No results</p>';
        return;
    }

    // Calculate stats
    let totalMatches = 0;
    let successCount = 0;
    let errorCount = 0;
    let emptyCount = 0;

    batchResults.forEach(result => {
        if (result.error) {
            errorCount++;
        } else if (result.results.length === 0) {
            emptyCount++;
        } else {
            successCount++;
            totalMatches += result.results.length;
        }
    });

    // Create summary
    const summary = document.createElement('div');
    summary.className = 'batch-results-summary';
    summary.innerHTML = `
        <span class="summary-text">${batchResults.length} images searched</span>
        <div class="summary-stats">
            <span class="stat-item stat-success">${successCount} with matches (${totalMatches} total)</span>
            ${emptyCount > 0 ? `<span class="stat-item stat-empty">${emptyCount} no matches</span>` : ''}
            ${errorCount > 0 ? `<span class="stat-item stat-error">${errorCount} failed</span>` : ''}
        </div>
    `;
    resultsContainer.appendChild(summary);

    // Create container for groups
    const groupsContainer = document.createElement('div');
    groupsContainer.className = 'batch-results-container';
    resultsContainer.appendChild(groupsContainer);

    // Create result groups
    batchResults.forEach((result, index) => {
        const group = document.createElement('div');
        group.className = 'batch-result-group';

        // Find matching file for thumbnail
        const matchingFile = selectedImageFiles.find(f => f.name === result.queryImage);

        // Header
        const header = document.createElement('div');
        header.className = 'batch-result-header';
        header.innerHTML = `
            <div class="query-thumbnail" id="thumb-${index}"></div>
            <div class="query-info">
                <div class="query-name">${result.queryImage}</div>
                <div class="match-count">${result.error ? 'Error' : (result.results.length === 0 ? 'No matches' : `${result.results.length} match${result.results.length > 1 ? 'es' : ''}`)}</div>
            </div>
            <span class="toggle-icon">&#9660;</span>
        `;

        // Load thumbnail
        if (matchingFile) {
            const reader = new FileReader();
            reader.onload = function(e) {
                const thumbDiv = document.getElementById(`thumb-${index}`);
                if (thumbDiv) {
                    thumbDiv.innerHTML = `<img src="${e.target.result}" alt="${result.queryImage}">`;
                }
            };
            reader.readAsDataURL(matchingFile);
        }

        // Content
        const content = document.createElement('div');
        content.className = 'batch-result-content';

        if (result.error) {
            content.innerHTML = `
                <div class="batch-result-error">
                    <span class="error-icon">&#9888;</span>
                    <span>${result.error}</span>
                </div>
            `;
        } else if (result.results.length === 0) {
            content.innerHTML = '<div class="batch-result-empty">No similar images found</div>';
        } else {
            // Create results table
            const resultsHTML = result.results.map(match => `
                <div class="result-item">
                    <div class="result-preview" onclick="openFile('${match.path.replace(/'/g, "\\'")}')">
                        <img src="/api/file?path=${encodeURIComponent(match.path)}&thumbnail=true"
                             alt="Result"
                             onerror="this.style.display='none'; this.parentElement.innerHTML='<span style=\\"color:#999;font-size:12px\\">No preview</span>'">
                    </div>
                    <div class="result-path" title="${match.path}" onclick="openFile('${match.path.replace(/'/g, "\\'")}')">${match.path}</div>
                    <div class="result-score">${match.score.toFixed(2)}</div>
                    <div class="result-actions">
                        <button class="action-btn copy" onclick="copyPath('${match.path.replace(/'/g, "\\'")}', this)">Copy</button>
                    </div>
                </div>
            `).join('');
            content.innerHTML = resultsHTML;
        }

        // Toggle functionality
        header.onclick = () => {
            header.classList.toggle('collapsed');
            content.classList.toggle('hidden');
        };

        group.appendChild(header);
        group.appendChild(content);
        groupsContainer.appendChild(group);
    });
}

function showError(message) {
    removeMessages();
    const errorDiv = document.createElement('div');
    errorDiv.className = 'error-message';
    errorDiv.textContent = message;
    document.querySelector('.left-panel').appendChild(errorDiv);
    setTimeout(() => errorDiv.remove(), 5000);
}

function showSuccess(message) {
    removeMessages();
    const successDiv = document.createElement('div');
    successDiv.className = 'success-message';
    successDiv.textContent = message;
    document.querySelector('.left-panel').appendChild(successDiv);
    setTimeout(() => successDiv.remove(), 5000);
}

function removeMessages() {
    document.querySelectorAll('.error-message, .success-message').forEach(el => el.remove());
}

// Initialize on load
document.addEventListener('DOMContentLoaded', function() {
    updateThreshold();
    loadConfig();
    
    // Update database info when path changes
    const dbPathInput = document.getElementById('dbPath');
    dbPathInput.addEventListener('blur', updateDatabaseInfo);
    
    // Initial database info update
    updateDatabaseInfo();
});

// Load configuration from server
async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        if (response.ok) {
            config = await response.json();
            // Config is already loaded via template, but we store it for later use
        }
    } catch (error) {
        console.error('Failed to load config:', error);
    }
}

// Save configuration to server
async function saveConfig() {
    const newConfig = {
        databasePath: document.getElementById('dbPath').value,
        folderPath: document.getElementById('folderPath').value,
        threshold: parseFloat(document.getElementById('thresholdValue').textContent),
        prefix: document.getElementById('prefix').value,
        forceRewrite: document.getElementById('forceRewrite').checked
    };
    
    try {
        await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(newConfig)
        });
    } catch (error) {
        console.error('Failed to save config:', error);
    }
}

// Update database info display
async function updateDatabaseInfo() {
    const dbPath = document.getElementById('dbPath').value;
    const dbInfo = document.getElementById('dbInfo');
    
    if (!dbPath) {
        dbInfo.innerHTML = '';
        return;
    }
    
    try {
        const response = await fetch(`/api/database-info?path=${encodeURIComponent(dbPath)}`);
        const info = await response.json();
        
        if (info.exists) {
            dbInfo.className = 'db-info exists';
            dbInfo.innerHTML = `Database exists - ${info.count} images`;
        } else {
            dbInfo.className = 'db-info';
            dbInfo.innerHTML = 'Database will be created';
        }
    } catch (error) {
        dbInfo.className = 'db-info';
        dbInfo.innerHTML = 'Error checking database';
    }
}

// Open file in system default application
function openFile(path) {
    // Since we can't directly open files from web browser,
    // we'll open the full-size image in a new tab
    window.open(`/api/file?path=${encodeURIComponent(path)}`, '_blank');
}

// Copy path to clipboard
async function copyPath(path, button) {
    try {
        await navigator.clipboard.writeText(path);
        button.textContent = 'Copied!';
        button.classList.add('copied');
        setTimeout(() => {
            button.textContent = 'Copy';
            button.classList.remove('copied');
        }, 2000);
    } catch (error) {
        console.error('Failed to copy:', error);
        // Fallback for older browsers
        const textArea = document.createElement('textarea');
        textArea.value = path;
        document.body.appendChild(textArea);
        textArea.select();
        try {
            document.execCommand('copy');
            button.textContent = 'Copied!';
            button.classList.add('copied');
            setTimeout(() => {
                button.textContent = 'Copy';
                button.classList.remove('copied');
            }, 2000);
        } catch (e) {
            alert('Failed to copy path');
        }
        document.body.removeChild(textArea);
    }
}