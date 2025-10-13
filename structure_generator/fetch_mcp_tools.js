#!/usr/bin/env node

/**
 * Script to fetch tools from an MCP server via stdio
 * Usage: node fetch_mcp_tools.js <server-package> <output-file>
 * Example: node fetch_mcp_tools.js @modelcontextprotocol/server-everything everything_tools.json
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const serverPackage = process.argv[2];
const outputFile = process.argv[3];

if (!serverPackage || !outputFile) {
  console.error('Usage: node fetch_mcp_tools.js <server-package> <output-file>');
  console.error('Example: node fetch_mcp_tools.js @modelcontextprotocol/server-everything everything_tools.json');
  process.exit(1);
}

console.log(`Fetching tools from ${serverPackage}...`);

// Spawn the MCP server
const server = spawn('npx', ['-y', serverPackage], {
  stdio: ['pipe', 'pipe', 'inherit']
});

let responseData = '';
let initializeRequestSent = false;
let toolsRequestSent = false;

// Send initialize request first
const initializeRequest = {
  jsonrpc: '2.0',
  id: 1,
  method: 'initialize',
  params: {
    protocolVersion: '2024-11-05',
    capabilities: {},
    clientInfo: {
      name: 'mcp-tool-fetcher',
      version: '1.0.0'
    }
  }
};

// Listen for data from server
server.stdout.on('data', (data) => {
  responseData += data.toString();

  // Try to parse JSON-RPC responses (they might be line-delimited)
  const lines = responseData.split('\n');

  for (let i = 0; i < lines.length - 1; i++) {
    const line = lines[i].trim();
    if (!line) continue;

    try {
      const response = JSON.parse(line);

      // Check if this is the initialize response
      if (response.id === 1 && !toolsRequestSent) {
        console.log('Received initialize response, requesting tools/list...');

        // Send tools/list request
        const toolsRequest = {
          jsonrpc: '2.0',
          id: 2,
          method: 'tools/list',
          params: {}
        };

        server.stdin.write(JSON.stringify(toolsRequest) + '\n');
        toolsRequestSent = true;
      }

      // Check if this is the tools/list response
      if (response.id === 2 && response.result) {
        console.log(`Found ${response.result.tools.length} tools`);

        // Extract server name from package name
        const serverName = serverPackage.split('/').pop().replace('server-', '');

        // Create output structure
        const output = {
          serverName: serverName,
          tools: response.result.tools
        };

        // Write to output file
        const outputPath = path.join(__dirname, 'tests', 'test_data', outputFile);
        fs.writeFileSync(outputPath, JSON.stringify(output, null, 2));

        console.log(`✓ Tools saved to ${outputPath}`);

        // Close the server
        server.kill();
        process.exit(0);
      }
    } catch (e) {
      // Not valid JSON yet, continue accumulating
    }
  }

  // Keep the last incomplete line
  responseData = lines[lines.length - 1];
});

// Handle server errors
server.on('error', (err) => {
  console.error('Failed to start server:', err);
  process.exit(1);
});

// Handle server exit
server.on('exit', (code) => {
  if (code !== 0) {
    console.error(`Server exited with code ${code}`);
    process.exit(code);
  }
});

// Send initialize request after a brief delay
setTimeout(() => {
  console.log('Sending initialize request...');
  server.stdin.write(JSON.stringify(initializeRequest) + '\n');
  initializeRequestSent = true;
}, 100);

// Timeout after 10 seconds
setTimeout(() => {
  console.error('Timeout waiting for server response');
  server.kill();
  process.exit(1);
}, 10000);
