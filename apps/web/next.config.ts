import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Dashboard pages fetch the API with session cookies at request time.
  // Cache Components prerender rejects that unless every route is rewritten
  // around Suspense/"use cache"; keep the app request-rendered instead.
  cacheComponents: false,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
