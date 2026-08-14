/** @type {import('next').NextConfig} */
const nextConfig = {
  transpilePackages: ["@hoptrace/shared-types"],
  async rewrites() {
    const api = process.env.HTTPTAP_API_URL || "http://127.0.0.1:8080";
    return [
      {
        source: "/api/:path*",
        destination: `${api}/:path*`,
      },
    ];
  },
};

export default nextConfig;
