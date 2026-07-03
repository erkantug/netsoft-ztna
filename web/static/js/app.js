// Netsoft ZTNA Wizard - Application JS

// HTMX extension for form validation
htmx.defineExtension('response-targets', {
    onEvent: function(name, evt) {
        if (name === 'htmx:beforeSwap') {
            // Allow 4xx responses to swap normally
            if (evt.detail.xhr.status >= 400) {
                evt.detail.shouldSwap = true;
            }
        }
    }
});

// Deploy function
function startDeploy() {
    document.getElementById('deploy-log').classList.remove('hidden');
    const logOutput = document.getElementById('log-output');
    const deployBtn = event.target;
    deployBtn.disabled = true;
    deployBtn.textContent = '⏳ Deploying...';

    // Start SSE log stream
    const evtSource = new EventSource('/api/deploy/log');
    evtSource.onmessage = function(event) {
        const line = document.createElement('div');
        line.className = 'text-green-400 text-xs';
        line.textContent = '> ' + event.data;
        logOutput.appendChild(line);
        logOutput.scrollTop = logOutput.scrollHeight;
    };
    evtSource.addEventListener('done', function() {
        evtSource.close();
        document.getElementById('deploy-complete').classList.remove('hidden');
        deployBtn.textContent = '✅ Done';
    });

    // Trigger deploy
    fetch('/api/deploy', { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                const line = document.createElement('div');
                line.className = 'text-red-400 text-xs';
                line.textContent = 'ERROR: ' + data.error;
                logOutput.appendChild(line);
            }
        });
}

// Password confirmation validation
document.addEventListener('htmx:beforeSwap', function(evt) {
    const step3Form = evt.target.closest('form[hx-post*="/api/step/3"]');
    if (step3Form) {
        const password = step3Form.querySelector('input[name="password"]').value;
        const confirm = step3Form.querySelector('input[name="password_confirm"]').value;
        if (password !== confirm) {
            evt.preventDefault();
            alert('Passwords do not match!');
            return;
        }
        // Remove confirm from form data before sending
        step3Form.querySelector('input[name="password_confirm"]').disabled = true;
    }
});

// Fade in new step content
document.addEventListener('htmx:afterSwap', function(evt) {
    if (evt.detail.target.id === 'step-content') {
        evt.detail.target.style.animation = 'none';
        evt.detail.target.offsetHeight; // reflow
        evt.detail.target.style.animation = 'fadeIn 0.3s ease-in';
    }
});

// Auto-detect IP on step 1 load
document.addEventListener('htmx:afterOnLoad', function(evt) {
    const domainField = document.querySelector('input[name="domain"]');
    if (domainField && !domainField.value) {
        // Suggest hostname
        const hostname = window.location.hostname;
        if (hostname && hostname !== 'localhost' && hostname !== '127.0.0.1') {
            domainField.placeholder = hostname + ' (detected)';
        }
    }
});
