let selectedImageFile = null;

function updateThreshold() {
    const slider = document.getElementById('threshold');
    const value = slider.value / 100;
    document.getElementById('thresholdValue').textContent = value.toFixed(2);
}

function browsePath(type) {
    // In a real implementation, this would open a file dialog
    // For now, we'll just show an alert
    alert('File browsing would be implemented here. For now, please type the path manually.');
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

    if (!dbPath || !folderPath) {
        alert('Please provide both database path and folder path');
        return;
    }

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
                folderPath: folderPath
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
        
        resultItem.innerHTML = `
            <div class="result-preview">
                <img src="/api/file?path=${encodeURIComponent(result.path)}" 
                     alt="Result" 
                     onerror="this.style.display='none'; this.parentElement.innerHTML='<span style=\\"color:#999;font-size:12px\\">No preview</span>'">
            </div>
            <div class="result-path" title="${result.path}">${result.path}</div>
            <div class="result-score">${result.score.toFixed(2)}</div>
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
});