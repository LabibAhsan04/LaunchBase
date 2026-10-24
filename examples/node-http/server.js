const http = require("http");

const port = Number(process.env.PORT || 3000);

const server = http.createServer((req, res) => {
  res.writeHead(200, { "content-type": "text/plain" });
  res.end("ok\n");
});

server.listen(port, "0.0.0.0");
