const http = require("http");
const { Storage } = require("megajs");
const { createClient } = require("redis");

const MAX_SIZE = 5 * 1024 * 1024 * 1024; // 5GB

// ===== REDIS TLS =====
const redis = createClient({
  url: process.env.REDIS_URL,
  socket: {
    tls: true,
    rejectUnauthorized: false
  }
});

redis.on("error", (err) => console.error("Redis error:", err));

(async () => {
  await redis.connect();
  console.log("✅ Redis connected");
})();

// ===== MEGA LOGIN =====
const storage = new Storage({
  email: process.env.MEGA_EMAIL,
  password: process.env.MEGA_PASSWORD
});

storage.on("ready", () => {
  console.log("✅ MEGA Logged In");
});

storage.on("error", err => {
  console.error("Mega login error:", err);
});

// Wait until login ready
async function ensureLogin() {
  if (!storage.root) {
    await new Promise(resolve => storage.once("ready", resolve));
  }
}

// ===== SERVER =====
const server = http.createServer(async (req, res) => {

  const urlObj = new URL(req.url, `http://${req.headers.host}`);
  const megaUrl = urlObj.searchParams.get("url");

  if (urlObj.pathname === "/") {
    return res.end("🚀 Enterprise MEGA Proxy Running");
  }

  if (!megaUrl) {
    res.writeHead(400);
    return res.end("Missing MEGA URL");
  }

  try {
    await ensureLogin();

    // ===== METADATA =====
    if (urlObj.pathname === "/api") {

      const cached = await redis.get(megaUrl);
      if (cached) {
        return res.end(JSON.stringify({
          status: "success",
          size: Number(cached),
          cached: true
        }));
      }

      const file = storage.fromURL(megaUrl);
      await file.loadAttributes();

      if (file.size > MAX_SIZE) {
        return res.end(JSON.stringify({
          status: "error",
          error: "File exceeds 5GB limit"
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

      const file = storage.fromURL(megaUrl);
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

      stream.on("error", (err) => {
        console.error("Stream error:", err);
        res.destroy();
      });

      stream.pipe(res);
      return;
    }

    res.writeHead(404);
    res.end("Not Found");

  } catch (err) {
    console.error("🔥 ERROR:", err.message);
    res.writeHead(500);
    res.end(JSON.stringify({
      status: "error",
      error: err.message
    }));
  }
});

const PORT = process.env.PORT || 8080;
server.listen(PORT, () => {
  console.log("🚀 Server running on port", PORT);
});
