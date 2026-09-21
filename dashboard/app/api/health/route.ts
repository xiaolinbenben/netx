export function GET() {
  return Response.json({
    service: "netx-dashboard",
    status: "ok",
    timestamp: new Date().toISOString(),
  });
}
