const documentPath = new URL("../.openapi-docs/index.html", import.meta.url);
const document = Bun.file(documentPath);

if (!(await document.exists())) {
  throw new Error("OpenAPI HTML is missing. Run bun run build:openapi first.");
}

const server = Bun.serve({
  hostname: "127.0.0.1",
  port: 8081,
  fetch() {
    return new Response(document, {
      headers: { "content-type": "text/html; charset=utf-8" },
    });
  },
});

console.log(`OpenAPI documentation: ${server.url}`);
