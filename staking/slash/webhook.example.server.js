const http = require('http');

const PORT = 5430;

const now = () => new Date().toString();

http
  .createServer(async (req, resp) => {
    let body = [];
    req
      .on('data', chunk => body.push(chunk))
      .on('end', () => {
        const result = JSON.parse(Buffer.concat(body).toString());
        // make it easy to read
        const at = `received at ${now()}`;
        console.log(at, JSON.stringify(result, null, 2));
        resp.end('hello world');
      });
  })
  .listen(PORT, () => {
    console.log(`started at ${now()} on port ${PORT}`);
  });
