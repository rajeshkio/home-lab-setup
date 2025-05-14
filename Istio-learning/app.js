const express = require('express');
const axios = require('axios');
const os = require('os');
const cors = require('cors');
const bodyParser = require('body-parser');

const app = express();
const port = process.env.PORT || 3000;

// Middleware
app.use(cors());
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// App configuration
const appVersion = process.env.APP_VERSION || 'v1';
const appColor = process.env.APP_COLOR || '#4285F4';
const appBehavior = process.env.APP_BEHAVIOR || 'normal'; // normal, slow, error
const apiBaseUrl = process.env.API_BASE_URL || 'http://localhost:3000';

// System information
const podName = process.env.HOSTNAME || 'unknown';
const podIp = Object.values(os.networkInterfaces())
  .flat()
  .filter(item => !item.internal && item.family === 'IPv4')
  .map(item => item.address)[0] || 'unknown';
const startTime = new Date();

// Artificial delays and errors based on configuration
const simulateLatency = (req, res, next) => {
  if (appBehavior === 'slow') {
    const delay = parseInt(process.env.DELAY_MS || '1000');
    setTimeout(next, delay);
  } else {
    next();
  }
};

// Setup routes
app.get('/health', (req, res) => {
  if (appBehavior === 'error' && req.query.checkType === 'deep') {
    return res.status(500).send('Health check failed');
  }
  res.status(200).send('Healthy');
});

app.get('/ready', (req, res) => {
  res.status(200).send('Ready');
});

// API endpoint that returns system and request information
app.get('/api/info', simulateLatency, (req, res) => {
  if (appBehavior === 'error') {
    return res.status(500).json({ error: 'Simulated server error' });
  }

  const info = {
    version: appVersion,
    color: appColor,
    podName: podName,
    podIp: podIp,
    startTime: startTime.toISOString(),
    uptime: Math.floor((new Date() - startTime) / 1000),
    timestamp: new Date().toISOString(),
    headers: req.headers,
    query: req.query,
    environment: process.env.NODE_ENV || 'development',
    behavior: appBehavior
  };

  res.json(info);
});

// Endpoint to test chained requests to demonstrate distributed tracing
app.get('/api/chain', simulateLatency, async (req, res) => {
  if (appBehavior === 'error') {
    return res.status(500).json({ error: 'Simulated server error' });
  }

  try {
    // Only attempt to call the chain if we're not going to create an infinite loop
    let chainResponse = null;
    const chainDepth = parseInt(req.query.depth || '0');

    if (chainDepth > 0) {
      const response = await axios.get(`${apiBaseUrl}/api/chain?depth=${chainDepth - 1}`, {
        headers: {
          'X-Request-ID': req.headers['x-request-id'] || 'unknown',
          'X-B3-TraceId': req.headers['x-b3-traceid'] || 'unknown',
          'X-B3-SpanId': req.headers['x-b3-spanid'] || 'unknown'
        }
      });
      chainResponse = response.data;
    }

    const result = {
      service: `istio-tester-${appVersion}`,
      podName: podName,
      timestamp: new Date().toISOString(),
      chainResponse: chainResponse,
      depth: chainDepth
    };

    res.json(result);
  } catch (error) {
    res.status(500).json({
      error: 'Error in chain call',
      message: error.message,
      service: `istio-tester-${appVersion}`,
      podName: podName
    });
  }
});

// Endpoint to test circuit breaking
app.get('/api/load', simulateLatency, (req, res) => {
  // Create some CPU load if requested
  const loadFactor = parseInt(req.query.factor || '0');
  let result = 0;

  if (loadFactor > 0) {
    const startTime = new Date().getTime();
    while (new Date().getTime() < startTime + loadFactor) {
      result += Math.random() * Math.random();
    }
  }

  if (appBehavior === 'error') {
    return res.status(500).json({ error: 'Simulated server error', loadResult: result });
  }

  res.json({
    service: `istio-tester-${appVersion}`,
    podName: podName,
    timestamp: new Date().toISOString(),
    loadFactor: loadFactor,
    result: result
  });
});

// Endpoint to test JWT auth
app.get('/api/secure', (req, res) => {
  // This endpoint will be secured by Istio Authentication Policy
  const jwt = req.headers.authorization || 'No JWT provided';

  res.json({
    service: `istio-tester-${appVersion}`,
    podName: podName,
    timestamp: new Date().toISOString(),
    message: 'Access to secure endpoint successful',
    jwtInfo: jwt.startsWith('Bearer ') ? 'Valid Bearer token found' : 'No valid Bearer token'
  });
});

// Endpoint to modify app behavior at runtime
app.post('/api/config', (req, res) => {
  const { behavior, delay } = req.body;

  if (behavior) {
    process.env.APP_BEHAVIOR = behavior;
  }

  if (delay) {
    process.env.DELAY_MS = delay.toString();
  }

  res.json({
    message: 'Configuration updated',
    currentConfig: {
      behavior: process.env.APP_BEHAVIOR,
      delay: process.env.DELAY_MS
    }
  });
});

// Main UI
app.get('/', (req, res) => {
  // Query parameters for the request
  const queryParams = Object.keys(req.query).map(key => `${key}=${req.query[key]}`).join('&');
  const queryString = queryParams ? `?${queryParams}` : '';

  const html = `
  <!DOCTYPE html>
  <html>
  <head>
    <title>Istio Tester - ${appVersion}</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
      body {
        font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
        margin: 0;
        padding: 0;
        background-color: #f5f5f5;
        color: #333;
      }
      .container {
        max-width: 1000px;
        margin: 0 auto;
        background-color: white;
        border-radius: 8px;
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
        overflow: hidden;
        margin-top: 20px;
        margin-bottom: 20px;
      }
      .header {
        background-color: ${appColor};
        color: white;
        padding: 20px;
        text-align: center;
      }
      .version-badge {
        display: inline-block;
        background-color: rgba(255, 255, 255, 0.3);
        padding: 5px 15px;
        border-radius: 20px;
        font-weight: bold;
        margin-top: 10px;
      }
      .behavior-badge {
        display: inline-block;
        background-color: rgba(0, 0, 0, 0.2);
        padding: 5px 15px;
        border-radius: 20px;
        font-size: 12px;
        margin-left: 10px;
      }
      .content {
        padding: 20px;
      }
      .panel {
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 15px;
        margin-bottom: 20px;
        background-color: #f9f9f9;
      }
      .panel-title {
        margin-top: 0;
        border-bottom: 1px solid #eee;
        padding-bottom: 10px;
        font-size: 16px;
      }
      .counter {
        text-align: center;
        font-size: 18px;
        margin: 20px 0;
      }
      button {
        background-color: ${appColor};
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
        margin: 5px;
        transition: background-color 0.2s;
      }
      button:hover {
        opacity: 0.9;
      }
      button:disabled {
        background-color: #cccccc;
        cursor: not-allowed;
        opacity: 0.7;
      }
      .button-group {
        text-align: center;
        margin: 15px 0;
      }
      table {
        width: 100%;
        border-collapse: collapse;
      }
      th, td {
        padding: 8px;
        text-align: left;
        border-bottom: 1px solid #ddd;
        font-size: 14px;
      }
      th {
        background-color: #f2f2f2;
        font-weight: normal;
      }
      .nav-tabs {
        display: flex;
        border-bottom: 1px solid #ddd;
        margin-bottom: 15px;
      }
      .nav-tab {
        padding: 10px 15px;
        cursor: pointer;
        border: 1px solid transparent;
        border-bottom: none;
        border-radius: 4px 4px 0 0;
        margin-right: 5px;
        background-color: #f9f9f9;
      }
      .nav-tab.active {
        background-color: white;
        border-color: #ddd;
        border-bottom-color: white;
        margin-bottom: -1px;
      }
      .tab-content {
        display: none;
      }
      .tab-content.active {
        display: block;
      }
      .footer {
        text-align: center;
        margin-top: 20px;
        color: #666;
        padding: 10px;
        font-size: 12px;
      }
      pre {
        background-color: #f5f5f5;
        padding: 10px;
        border-radius: 4px;
        overflow-x: auto;
        font-size: 13px;
      }
      .input-group {
        margin-bottom: 10px;
      }
      .input-group label {
        display: block;
        margin-bottom: 5px;
      }
      .input-group input, .input-group select {
        width: 100%;
        padding: 8px;
        border: 1px solid #ddd;
        border-radius: 4px;
      }
      .loading {
        display: inline-block;
        width: 20px;
        height: 20px;
        border: 3px solid rgba(255,255,255,.3);
        border-radius: 50%;
        border-top-color: #fff;
        animation: spin 1s ease-in-out infinite;
        margin-left: 10px;
        vertical-align: middle;
      }
      @keyframes spin {
        to { transform: rotate(360deg); }
      }
      .hidden {
        display: none;
      }
      .success {
        color: #28a745;
        font-weight: bold;
      }
      .error {
        color: #dc3545;
        font-weight: bold;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <div class="header">
        <h1>Istio Tester</h1>
        <div class="version-badge">${appVersion}</div>
        <div class="behavior-badge">Behavior: ${appBehavior}</div>
      </div>
      
      <div class="content">
        <div class="nav-tabs">
          <div class="nav-tab active" data-tab="traffic">Traffic Routing</div>
          <div class="nav-tab" data-tab="resilience">Resilience Testing</div>
          <div class="nav-tab" data-tab="security">Security</div>
          <div class="nav-tab" data-tab="observability">Observability</div>
          <div class="nav-tab" data-tab="config">Configuration</div>
        </div>
        
        <!-- Traffic Routing Tab -->
        <div class="tab-content active" id="traffic-tab">
          <div class="panel">
            <h3 class="panel-title">Pod Information</h3>
            <table>
              <tr>
                <th width="30%">Pod Name:</th>
                <td>${podName}</td>
              </tr>
              <tr>
                <th>Pod IP:</th>
                <td>${podIp}</td>
              </tr>
              <tr>
                <th>Version:</th>
                <td>${appVersion}</td>
              </tr>
              <tr>
                <th>Behavior:</th>
                <td>${appBehavior}</td>
              </tr>
              <tr>
                <th>Uptime:</th>
                <td><span id="uptime">0</span> seconds</td>
              </tr>
            </table>
          </div>
          
          <div class="panel">
            <h3 class="panel-title">Request Information</h3>
            <table>
              <tr>
                <th width="30%">Query Parameters:</th>
                <td>${queryString || 'None'}</td>
              </tr>
              <tr>
                <th>User-Agent:</th>
                <td>${req.headers['user-agent'] || 'Not provided'}</td>
              </tr>
            </table>
            
            <div class="button-group">
              <button id="add-version-param">Add version=v2 Parameter</button>
              <button id="add-custom-header">Add Custom Header</button>
            </div>
          </div>
          
          <div class="counter">
            <p>Requests made: <span id="count">0</span></p>
          </div>
          
          <div class="button-group">
            <button id="refreshBtn">Refresh</button>
            <button id="autoRefreshBtn">Auto Refresh (1s)</button>
            <button id="stopBtn" disabled>Stop</button>
          </div>
          
          <div class="panel">
            <h3 class="panel-title">Traffic Distribution Log</h3>
            <table id="trafficLog">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Version</th>
                  <th>Pod</th>
                  <th>Behavior</th>
                </tr>
              </thead>
              <tbody>
                <!-- Log entries will be added here -->
              </tbody>
            </table>
          </div>
        </div>
        
        <!-- Resilience Testing Tab -->
        <div class="tab-content" id="resilience-tab">
          <div class="panel">
            <h3 class="panel-title">Resilience Testing</h3>
            <p>Test how Istio handles service failures, latency, and circuit breaking.</p>
            
            <div class="button-group">
              <button id="testTimeoutBtn">Test Timeout</button>
              <button id="testCircuitBreakerBtn">Test Circuit Breaker</button>
              <button id="testFaultInjectionBtn">Test Fault Injection</button>
            </div>
            
            <div class="input-group">
              <label for="chainDepth">Chain Depth (Tracing Test):</label>
              <input type="number" id="chainDepth" min="0" max="10" value="3">
              <button id="testChainBtn">Test Service Chain</button>
            </div>
            
            <div class="input-group">
              <label for="loadFactor">Load Factor (Circuit Breaker Test):</label>
              <input type="number" id="loadFactor" min="0" max="5000" value="1000">
              <button id="generateLoadBtn">Generate Load</button>
            </div>
            
            <h4>Results:</h4>
            <pre id="resilienceResults">No tests run yet.</pre>
          </div>
        </div>
        
        <!-- Security Tab -->
        <div class="tab-content" id="security-tab">
          <div class="panel">
            <h3 class="panel-title">Security Testing</h3>
            <p>Test Istio authentication and authorization policies.</p>
            
            <div class="input-group">
              <label for="jwtToken">JWT Token (for Testing):</label>
              <input type="text" id="jwtToken" placeholder="Enter JWT token">
            </div>
            
            <div class="button-group">
              <button id="testAuthBtn">Test Authentication</button>
              <button id="testMTLSBtn">Test mTLS</button>
            </div>
            
            <h4>Results:</h4>
            <pre id="securityResults">No tests run yet.</pre>
          </div>
        </div>
        
        <!-- Observability Tab -->
        <div class="tab-content" id="observability-tab">
          <div class="panel">
            <h3 class="panel-title">Observability</h3>
            <p>Examine request headers and tracing information.</p>
            
            <h4>Request Headers:</h4>
            <pre id="requestHeaders">Click "Show Headers" to view request headers</pre>
            
            <div class="button-group">
              <button id="showHeadersBtn">Show Headers</button>
              <button id="testTracingBtn">Generate Trace</button>
            </div>
            
            <h4>Tracing Information:</h4>
            <pre id="tracingInfo">No tracing information available.</pre>
          </div>
        </div>
        
        <!-- Configuration Tab -->
        <div class="tab-content" id="config-tab">
          <div class="panel">
            <h3 class="panel-title">Service Configuration</h3>
            <p>Modify service behavior to test different Istio features.</p>
            
            <div class="input-group">
              <label for="behaviorSelect">Service Behavior:</label>
              <select id="behaviorSelect">
                <option value="normal" ${appBehavior === 'normal' ? 'selected' : ''}>Normal</option>
                <option value="slow" ${appBehavior === 'slow' ? 'selected' : ''}>Slow (Latency)</option>
                <option value="error" ${appBehavior === 'error' ? 'selected' : ''}>Error (5xx)</option>
              </select>
            </div>
            
            <div class="input-group">
              <label for="delayMs">Delay (ms):</label>
              <input type="number" id="delayMs" min="0" max="10000" value="${process.env.DELAY_MS || '1000'}">
            </div>
            
            <div class="button-group">
              <button id="updateConfigBtn">Update Configuration</button>
              <span id="configUpdateStatus" class="hidden"></span>
            </div>
            
            <h4>Current Configuration:</h4>
            <pre id="currentConfig">
Behavior: ${appBehavior}
Delay: ${process.env.DELAY_MS || '1000'} ms
            </pre>
          </div>
        </div>
      </div>
      
      <div class="footer">
        <p>Istio Testing Tool - Pod: ${podName} | Version: ${appVersion}</p>
      </div>
    </div>

    <script>
      // Variables
      let count = 0;
      let autoRefreshInterval;
      const maxLogEntries = 10;
      const startTime = new Date();
      
      // Update uptime
      setInterval(() => {
        const uptime = Math.floor((new Date() - startTime) / 1000);
        document.getElementById('uptime').textContent = uptime;
      }, 1000);
      
      // Tab Navigation
      document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.addEventListener('click', () => {
          // Deactivate all tabs
          document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
          document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
          
          // Activate selected tab
          tab.classList.add('active');
          document.getElementById(tab.dataset.tab + '-tab').classList.add('active');
        });
      });
      
      // Function to update the counter
      function updateCounter() {
        count++;
        document.getElementById('count').textContent = count;
      }
      
      // Function to add a log entry
      function addLogEntry(version, podName, behavior) {
        const table = document.getElementById('trafficLog').getElementsByTagName('tbody')[0];
        const now = new Date().toISOString().split('T')[1].split('.')[0];
        
        // Create new row
        const row = table.insertRow(0);
        const timeCell = row.insertCell(0);
        const versionCell = row.insertCell(1);
        const podCell = row.insertCell(2);
        const behaviorCell = row.insertCell(3);
        
        timeCell.textContent = now;
        versionCell.textContent = version;
        podCell.textContent = podName;
        behaviorCell.textContent = behavior;
        
        // Limit the number of entries
        if (table.rows.length > maxLogEntries) {
          table.deleteRow(table.rows.length - 1);
        }
      }
      
      // Function to fetch data
      function fetchData() {
        // Get current URL and parameters
        const url = new URL(window.location.href);
        const params = new URLSearchParams(url.search);
        
        fetch('/api/info' + (url.search || ''))
          .then(response => {
            if (!response.ok) {
              throw new Error('Network response was not ok: ' + response.status);
            }
            return response.json();
          })
          .then(data => {
            updateCounter();
            addLogEntry(data.version, data.podName, data.behavior);
          })
          .catch(error => {
            console.error('Error fetching data:', error);
            // Still update counter and add an error entry
            updateCounter();
            addLogEntry('Error', 'Request failed', 'error');
          });
      }
      
      // Set up URL parameter buttons
      document.getElementById('add-version-param').addEventListener('click', () => {
        const url = new URL(window.location.href);
        url.searchParams.set('version', 'v2');
        window.location.href = url.toString();
      });
      
      document.getElementById('add-custom-header').addEventListener('click', () => {
        // Can't add custom headers directly via UI, so we'll tell the user how
        alert('To test header-based routing, use browser extensions like ModHeader to add custom headers like "x-user-type: premium"');
      });
      
      // Set up refresh buttons
      document.getElementById('refreshBtn').addEventListener('click', fetchData);
      
      // Set up auto refresh button
      document.getElementById('autoRefreshBtn').addEventListener('click', function() {
        if (!autoRefreshInterval) {
          fetchData(); // Fetch immediately
          autoRefreshInterval = setInterval(fetchData, 1000);
          this.disabled = true;
          document.getElementById('stopBtn').disabled = false;
        }
      });
      
      // Stop button
      document.getElementById('stopBtn').addEventListener('click', function() {
        if (autoRefreshInterval) {
          clearInterval(autoRefreshInterval);
          autoRefreshInterval = null;
          this.disabled = true;
          document.getElementById('autoRefreshBtn').disabled = false;
        }
      });
      
      // Resilience Testing
      document.getElementById('testTimeoutBtn').addEventListener('click', () => {
        const resultsElement = document.getElementById('resilienceResults');
        resultsElement.textContent = 'Testing timeout...';
        
        fetch('/api/info?delay=high')
          .then(response => response.json())
          .then(data => {
            resultsElement.textContent = JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 'Timeout error: ' + error.message;
          });
      });
      
      document.getElementById('testCircuitBreakerBtn').addEventListener('click', () => {
        const resultsElement = document.getElementById('resilienceResults');
        resultsElement.textContent = 'Testing circuit breaker...';
        
        // Generate many concurrent requests to trigger circuit breaker
        const requests = [];
        for (let i = 0; i < 20; i++) {
          requests.push(fetch('/api/info?concurrent=' + i));
        }
        
        Promise.allSettled(requests)
          .then(results => {
            const successful = results.filter(r => r.status === 'fulfilled').length;
            const failed = results.filter(r => r.status === 'rejected').length;

	    document.getElementById('resilienceResults').textContent =
            "Circuit Breaker Test Results:\n" +
            "Total Requests: " + requests.length + "\n" +
            "Successful: " + successful + "\n" +
            "Failed/Rejected: " + failed + "\n\n" +
            "If circuit breaking is configured correctly, some requests should fail when the circuit opens.";
          });
       });
            
      document.getElementById('testFaultInjectionBtn').addEventListener('click', () => {
        const resultsElement = document.getElementById('resilienceResults');
        resultsElement.textContent = 'Testing fault injection...';
        
        fetch('/api/info?triggerFault=true')
          .then(response => response.json())
          .then(data => {
            resultsElement.textContent = 
              'Fault injection response (if Istio fault rules are in place, this should show an error or delayed response):\\n\\n' +
              JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 
              'Fault injection triggered error (expected if fault injection is configured):\\n\\n' + error.message;
          });
      });
      
      document.getElementById('testChainBtn').addEventListener('click', () => {
        const depth = document.getElementById('chainDepth').value;
        const resultsElement = document.getElementById('resilienceResults');
        resultsElement.textContent = 'Testing service chain with depth ' + depth + '...';
        
        fetch('/api/chain?depth=' + depth)
          .then(response => response.json())
          .then(data => {
            resultsElement.textContent = JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 'Chain test error: ' + error.message;
          });
      });
      
      document.getElementById('generateLoadBtn').addEventListener('click', () => {
        const loadFactor = document.getElementById('loadFactor').value;
        const resultsElement = document.getElementById('resilienceResults');
        resultsElement.textContent = 'Generating load with factor ' + loadFactor + '...';
        
        fetch('/api/load?factor=' + loadFactor)
          .then(response => response.json())
          .then(data => {
            resultsElement.textContent = JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 'Load test error: ' + error.message;
          });
      });
      
      // Security Testing
      document.getElementById('testAuthBtn').addEventListener('click', () => {
        const token = document.getElementById('jwtToken').value;
        const headers = token ? { 'Authorization': 'Bearer ' + token } : {};
        const resultsElement = document.getElementById('securityResults');
        
        resultsElement.textContent = 'Testing authentication...';
        
        fetch('/api/secure', { headers })
          .then(response => {
            if (!response.ok) {
              throw new Error('Authentication failed: ' + response.status);
            }
            return response.json();
          })
          .then(data => {
            resultsElement.textContent = JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 'Authentication error: ' + error.message;
          });
      });
      
      document.getElementById('testMTLSBtn').addEventListener('click', () => {
        const resultsElement = document.getElementById('securityResults');
        resultsElement.textContent = 'Testing mTLS connection...';
        
        fetch('/api/info?checkMTLS=true')
          .then(response => response.json())
          .then(data => {
            // Get specific TLS headers that Istio adds
            const tlsInfo = 'mTLS is likely ' + 
              (data.headers['x-forwarded-client-cert'] ? 'ENABLED' : 'NOT ENABLED') +
              ' (based on presence of x-forwarded-client-cert header)';
            
            resultsElement.textContent = tlsInfo + '\\n\\n' + JSON.stringify(data, null, 2);
          })
          .catch(error => {
            resultsElement.textContent = 'mTLS test error: ' + error.message;
          });
      });
      
      // Observability Testing
      document.getElementById('showHeadersBtn').addEventListener('click', () => {
        const headersElement = document.getElementById('requestHeaders');
        headersElement.textContent = 'Loading headers...';
        
        fetch('/api/info')
          .then(response => response.json())
          .then(data => {
            headersElement.textContent = JSON.stringify(data.headers, null, 2);
          })
          .catch(error => {
            headersElement.textContent = 'Error fetching headers: ' + error.message;
          });
      });
      
      document.getElementById('testTracingBtn').addEventListener('click', () => {
        const tracingElement = document.getElementById('tracingInfo');
        tracingElement.textContent = 'Generating trace...';
        
        fetch('/api/chain?depth=3')
          .then(response => response.json())
          .then(data => {
            let tracingInfo = 'Trace generated. If Istio tracing is configured correctly, you should see this trace in your tracing dashboard.\\n\\n';
            tracingInfo += 'Tracing Headers Found:\\n';
            
            // Extract tracing headers
            const tracingHeaders = [
              'x-request-id',
              'x-b3-traceid',
              'x-b3-spanid',
              'x-b3-parentspanid',
              'x-b3-sampled',
              'x-b3-flags
	      'x-request-id',
              'x-b3-traceid',
              'x-b3-spanid',
              'x-b3-parentspanid',
              'x-b3-sampled',
              'x-b3-flags',
              'x-ot-span-context'
            ];
            tracingHeaders.forEach(header => {
              if (data.headers && data.headers[header]) {
                tracingInfo += header + ": " + data.headers[header] + "\n";
              }
            });

            tracingInfo += '\nService Chain Response:\n' + JSON.stringify(data, null, 2);
            tracingElement.textContent = tracingInfo;
          })
          .catch(error => {
            tracingElement.textContent = 'Tracing error: ' + error.message;
          });
      });

      // Configuration Tab
      document.getElementById('updateConfigBtn').addEventListener('click', () => {
        const behavior = document.getElementById('behaviorSelect').value;
        const delay = document.getElementById('delayMs').value;
        const statusElement = document.getElementById('configUpdateStatus');
        const configElement = document.getElementById('currentConfig');
        const button = document.getElementById('updateConfigBtn');

        // Show loading indicator
        statusElement.innerHTML = '<span class="loading"></span> Updating...';
        statusElement.classList.remove('hidden', 'success', 'error');
        button.disabled = true;

        fetch('/api/config', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ behavior, delay })
        })
        .then(response => {
          if (!response.ok) {
            throw new Error('Configuration update failed: ' + response.status);
          }
          return response.json();
        })
        .then(data => {
          // Update the displayed configuration
	  document.getElementById('currentConfig').textContent =
            "Behavior: " + data.currentConfig.behavior + "\n" +
             "Delay: " + data.currentConfig.delay + " ms";

          // Show success message
          statusElement.innerHTML = '✓ Configuration updated successfully!';
          statusElement.classList.add('success');

          // Make behavior badge reflect the new behavior
          const behaviorBadge = document.querySelector('.behavior-badge');
          if (behaviorBadge) {
            behaviorBadge.textContent = 'Behavior: ' + data.currentConfig.behavior;
          }
        })
        .catch(error => {
          console.error('Error:', error);
          statusElement.textContent = '✗ ' + error.message;
          statusElement.classList.add('error');
        })
        .finally(() => {
          button.disabled = false;

          // Hide status message after 3 seconds
          setTimeout(() => {
            statusElement.classList.add('hidden');
          }, 3000);
        });
      });

      // Automatic data loading on page load
      document.addEventListener('DOMContentLoaded', () => {
        fetchData();
      });

      // Prevent accidental navigation when auto-refresh is active
      window.addEventListener('beforeunload', (event) => {
        if (autoRefreshInterval) {
          // Cancel the event
          event.preventDefault();
          // Chrome requires returnValue to be set
          event.returnValue = '';

          // Show confirmation dialog
          return 'Auto-refresh is still active. Are you sure you want to leave?';
        }
      });
    </script>
  </body>
  </html>
  `;

  res.send(html);
});

app.listen(port, '0.0.0.0', () => {
  console.log('Istio Tester (' + appVersion + ') listening at http://0.0.0.0:' + port);
});
