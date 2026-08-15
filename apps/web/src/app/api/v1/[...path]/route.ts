import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const upstreamBase = () =>
  (
    process.env.HOPTRACE_API_URL ||
    process.env.HTTPTAP_API_URL ||
    "http://127.0.0.1:8080"
  ).replace(/\/$/, "");

async function proxy(req: NextRequest, path: string[]) {
  const targetPath = path.join("/");
  const url = new URL(req.url);
  const dest = `${upstreamBase()}/v1/${targetPath}${url.search}`;

  const headers = new Headers();
  const pass = ["content-type", "accept", "authorization", "x-api-key"];
  for (const name of pass) {
    const v = req.headers.get(name);
    if (v) headers.set(name, v);
  }

  // Server-side key (preferred for local -require-auth). Client header wins if set.
  const serverKey = process.env.HOPTRACE_API_KEY || "";
  if (serverKey && !headers.has("x-api-key") && !headers.has("authorization")) {
    headers.set("X-API-Key", serverKey);
  }

  const init: RequestInit = {
    method: req.method,
    headers,
    cache: "no-store",
  };

  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }

  let upstream: Response;
  try {
    upstream = await fetch(dest, init);
  } catch (e) {
    const msg = e instanceof Error ? e.message : "upstream unreachable";
    return NextResponse.json(
      { error: `hoptrace API unreachable at ${upstreamBase()}: ${msg}` },
      { status: 502 },
    );
  }

  const outHeaders = new Headers();
  const ct = upstream.headers.get("content-type");
  if (ct) outHeaders.set("content-type", ct);

  return new NextResponse(upstream.body, {
    status: upstream.status,
    headers: outHeaders,
  });
}

type Ctx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function POST(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function PUT(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function PATCH(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;
  return proxy(req, path);
}

export async function DELETE(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;
  return proxy(req, path);
}
