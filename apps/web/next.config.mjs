/** @type {import('next').NextConfig} */
const nextConfig = {
  transpilePackages: ["@hoptrace/shared-types"],
  // REST is proxied by src/app/api/v1/[...path]/route.ts (injects HOPTRACE_API_KEY).
};

export default nextConfig;
