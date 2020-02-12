const http = require('http');

http
  .createServer(async (req, resp) => {
    let body = [];
    req
      .on('data', chunk => body.push(chunk))
      .on('end', () => {
        body = Buffer.concat(body).toString();
        console.log(body);
        resp.end('hello world');
      });
  })
  .listen(5430, () => {
    console.log('started server');
  });
