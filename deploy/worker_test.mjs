// node --test deploy/worker_test.mjs
//
// The Worker against a bucket held in memory.
import { test } from "node:test";
import assert from "node:assert/strict";
import worker from "./worker.js";

const objects = {
	"index.html": "home",
	"fr/alg/index.html": "book",
	"katex/fonts/a.woff2": "font",
	"style.css": "css",
	"404.html": "missing",
};
const env = {
	SITE: {
		async get(k) {
			return k in objects ? { body: objects[k], httpEtag: '"e"' } : null;
		},
		async head(k) {
			return k in objects ? {} : null;
		},
	},
};

const get = (path, init) => worker.fetch(new Request("https://bourbaki.tamnd.com" + path, init), env);

test("a page is served from its directory, gzipped and public", async () => {
	const res = await get("/");
	assert.equal(res.status, 200);
	assert.equal(await res.text(), "home");
	assert.equal(res.headers.get("content-type"), "text/html; charset=utf-8");
	assert.equal(res.headers.get("content-encoding"), "gzip");
	assert.match(res.headers.get("cache-control"), /^public/);
	assert.equal(res.headers.get("x-robots-tag"), null);
});

test("a font is stored as it is and kept for a week", async () => {
	const res = await get("/katex/fonts/a.woff2");
	assert.equal(res.headers.get("content-encoding"), null);
	assert.equal(res.headers.get("cache-control"), "public, max-age=604800");
});

test("a page asked for without its slash is sent to it", async () => {
	const res = await get("/fr/alg?x=1");
	assert.equal(res.status, 301);
	assert.equal(res.headers.get("location"), "https://bourbaki.tamnd.com/fr/alg/?x=1");
});

test("an address with nothing at it gets the site's 404 page", async () => {
	const res = await get("/nothing/here/");
	assert.equal(res.status, 404);
	assert.equal(await res.text(), "missing");
	assert.equal((await get("/a/..%2Findex.html")).status, 404);
});

test("only GET and HEAD", async () => {
	assert.equal((await get("/", { method: "POST" })).status, 405);
	const head = await get("/", { method: "HEAD" });
	assert.equal(head.status, 200);
	assert.equal(await head.text(), "");
});
