let selectedImageFile = null;
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
    const file = event.target.files[0];
    if (file) {
        selectedImageFile = file;
        const reader = new FileReader();
        reader.onload = function(e) {
            document.getElementById('selectedImage').innerHTML = 
                `<img src="${e.target.result}" alt="Selected image">
                 <p style="font-size: 12px; margin-top: 5px;">${file.name}</p>`;
        };
        reader.readAsDataURL(file);
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

    if (!selectedImageFile) {
        alert('Please select an image to search');
        return;
    }
    
    // Save config
    saveConfig();

    // Show loading
    const resultsContainer = document.getElementById('results');
    resultsContainer.innerHTML = '<p style="text-align: center; color: #666;">Searching...</p>';

    // Prepare form data for upload
    const formData = new FormData();
    formData.append('image', selectedImageFile);
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