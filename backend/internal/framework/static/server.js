const http = require("http");
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "dist");
const port = Number(process.env.PORT || 3000);

function insideRoot(file) {
  const rel = path.relative(root, file);
  return rel === "" || (!rel.startsWith("..") && !path.isAbsolute(rel));
}

const server = http.createServer((req, res) => {
  const raw = (req.url || "/").split("?")[0];
  let requestPath = "/";
  try {
    requestPath = decodeURIComponent(raw);
  } catch {
    res.writeHead(400);
    res.end();
    return;
  }

  let file = path.join(root, "index.html");
  if (requestPath !== "/") {
    file = path.normalize(path.join(root, requestPath));
  }
  if (!insideRoot(file)) {
    res.writeHead(400);
    res.end();
    return;
  }

  fs.readFile(file, (err, body) => {
    if (!err) {
      res.writeHead(200);
      res.end(body);
      return;
    }
    fs.readFile(path.join(root, "index.html"), (fallbackErr, index) => {
      if (fallbackErr) {
        res.writeHead(404);
        res.end();
        return;
      }
      res.writeHead(200);
      res.end(index);
    });
  });
});

server.listen(port, "0.0.0.0");
