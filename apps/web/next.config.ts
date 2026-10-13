import type { NextConfig } from "next";

const apiProxyTarget = (
  process.env.API_PROXY_TARGET ?? "http://127.0.0.1:8080"
).replace(/\/$/, "");

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiProxyTarget}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
