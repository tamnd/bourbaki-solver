// The Worker that serves bourbaki.tamnd.com out of the R2 bucket a
// `bourbaki publish -gzip` build is uploaded to.
//
// The site is public and static. Every file but a font is stored gzipped, so
// the Worker hands the stored bytes over with Content-Encoding: gzip and never
// decompresses anything. A page is asked for by its directory and served from
// the index.html in it, and an address with no page gets the site's 404.html.

const types = {
	html: "text/html; charset=utf-8",
	css: "text/css; charset=utf-8",
	js: "text/javascript; charset=utf-8",
	json: "application/json; charset=utf-8",
	svg: "image/svg+xml",
	woff2: "font/woff2",
	txt: "text/plain; charset=utf-8",
	md: "text/markdown; charset=utf-8",
	xml: "application/xml; charset=utf-8",
};

// How long a browser and the edge may keep a file. The KaTeX assets and the
// fonts change only with a release of either; a page changes whenever a chapter
// is read again.
function cacheFor(key) {
	if (key.startsWith("katex/") || key.startsWith("fonts/")) {
		return "public, max-age=604800";
	}
	return "public, max-age=300, s-maxage=3600";
}

export default {
	async fetch(req, env) {
		if (req.method !== "GET" && req.method !== "HEAD") {
			return new Response("method not allowed\n", { status: 405, headers: { allow: "GET, HEAD" } });
		}

		const url = new URL(req.url);
		let key;
		try {
			key = decodeURIComponent(url.pathname).replace(/^\/+/, "");
		} catch {
			return notFound(env, req);
		}
		if (key.split("/").includes("..")) {
			return notFound(env, req);
		}
		if (key === "" || key.endsWith("/")) {
			key += "index.html";
		}

		let obj = await env.SITE.get(key, { onlyIf: req.headers });
		if (obj === null && !/\.[A-Za-z0-9]+$/.test(key)) {
			// A page asked for without its closing slash.
			if ((await env.SITE.head(key + "/index.html")) !== null) {
				return Response.redirect(url.origin + url.pathname + "/" + url.search, 301);
			}
		}
		if (obj === null) {
			return notFound(env, req);
		}
		const headers = headersFor(key, obj);
		if (!("body" in obj)) {
			// The object is the one the browser already has.
			return new Response(null, { status: 304, headers });
		}
		return new Response(req.method === "HEAD" ? null : obj.body, { headers, encodeBody: "manual" });
	},
};

function headersFor(key, obj) {
	const ext = key.slice(key.lastIndexOf(".") + 1).toLowerCase();
	const headers = new Headers({
		"content-type": types[ext] || "application/octet-stream",
		etag: obj.httpEtag,
		"cache-control": cacheFor(key),
		"referrer-policy": "strict-origin-when-cross-origin",
		"x-content-type-options": "nosniff",
	});
	// Everything but a font is stored gzipped, since that is what
	// `bourbaki publish -gzip` writes.
	if (ext !== "woff2") {
		headers.set("content-encoding", "gzip");
	}
	return headers;
}

async function notFound(env, req) {
	const obj = await env.SITE.get("404.html");
	if (obj === null) {
		return new Response("not found\n", { status: 404, headers: { "content-type": "text/plain" } });
	}
	const headers = headersFor("404.html", obj);
	headers.set("cache-control", "public, max-age=60");
	return new Response(req.method === "HEAD" ? null : obj.body, { status: 404, headers, encodeBody: "manual" });
}
