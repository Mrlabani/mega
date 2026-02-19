const http = require("http");
const { File } = require("megajs");
const { createClient } = require("redis");

const MAX_SIZE = 5 * 1024 * 1024 * 1024; // 5GB

// Redis TLS (Upstash)
const redis = createClient({
  url: process.env.REDIS_URL,
  socket: {
    tls: true,
    rejectUnauthorized: false
  }
});

redis.connect()
  .then(() => console.log("✅ Redis connected"))
  .catch(err => console.error("Redis error:", err));

const server = http.createServer(async (req, res) => {
  const urlObj = new URL(req.url, `http://${req.headers.host}`);
  const megaUrl = urlObj.searchParams.get("url");

  if (urlObj.pathname === "/") {
    return res.end("🚀 Mega Download Proxy Running");
  }

  if (!megaUrl) {
    res.writeHead(400);
    return res.end("Missing MEGA URL");
  }

  try {

    // ===== METADATA API =====
    if (urlObj.pathname === "/api") {

      const cached = await redis.get(megaUrl);
      if (cached) {
        return res.end(JSON.stringify({
          status: "success",
          size: Number(cached),
          cached: true
        }));
      }

      const file = File.fromURL(megaUrl);
      await file.loadAttributes();

      if (file.size > MAX_SIZE) {
        return res.end(JSON.stringify({
          status: "error",
          error: "File exceeds 5GB"
        }));
      }

      await redis.setEx(megaUrl, 3600, file.size.toString());

      return res.end(JSON.stringify({
        status: "success",
        name: file.name,
        size: file.size,
        download: `/download?url=${encodeURIComponent(megaUrl)}`
      }));
    }

    // ===== DOWNLOAD PROXY =====
    if (urlObj.pathname === "/download") {

      const file = File.fromURL(megaUrl);
      await file.loadAttributes();

      if (file.size > MAX_SIZE) {
        res.writeHead(400);
        return res.end("File exceeds 5GB limit");
      }

      res.writeHead(200, {
        "Content-Type": "application/octet-stream",
        "Content-Disposition": `attachment; filename="${file.name}"`,
        "Content-Length": file.size
      });

      const stream = file.download();
      stream.pipe(res);

      stream.on("error", () => {
        res.end();
      });

      return;
    }

    res.writeHead(404);
    res.end("Not Found");

  } catch (err) {
    res.writeHead(500);
    res.end("Error processing request");
  }
});

const PORT = process.env.PORT || 8080;
server.listen(PORT, () => {
  console.log("🚀 Server running on port", PORT);
});
