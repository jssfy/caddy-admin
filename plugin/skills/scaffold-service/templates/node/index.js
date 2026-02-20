const http = require('http');

const PORT = __BACKEND_PORT__;

const server = http.createServer((req, res) => {
  res.setHeader('Content-Type', 'application/json');

  if (req.url === '/health') {
    res.end(JSON.stringify({ status: 'ok', service: '__PROJECT_NAME__-backend' }));
  } else if (req.url === '/api/hello') {
    res.end(JSON.stringify({ message: 'Hello from __PROJECT_NAME__ API', port: String(PORT) }));
  } else {
    res.statusCode = 404;
    res.end(JSON.stringify({ error: 'not found' }));
  }
});

server.listen(PORT, () => console.log(`Listening on :${PORT}`));
