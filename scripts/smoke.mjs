// Smoke test for a running stack:   docker compose up -d --wait && node scripts/smoke.mjs [baseUrl]
// Needs Node 18+ and the demo data (SEED_ON_START=true). Exits non-zero on the first failure.
const base = process.argv[2] ?? "http://localhost:3000";

async function api(path, token, init = {}) {
  const res = await fetch(`${base}/api/v1${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...(token && { Authorization: `Bearer ${token}` }) },
  });
  return { status: res.status, body: await res.json().catch(() => null) };
}

function must(ok, what) {
  if (!ok) throw new Error(`FAILED: ${what}`);
  console.log(`ok  ${what}`);
}

async function login(email) {
  const r = await api("/auth/login", null, { method: "POST", body: JSON.stringify({ email, password: "Password123!" }) });
  must(r.status === 200 && !!r.body?.data?.access_token, `login as ${email}`);
  return r.body.data.access_token;
}

try {
  const page = await fetch(`${base}/`);
  must(page.status === 200 && (await page.text()).includes('id="root"'), "web UI is served");

  const alice = await login("alice@acme.test");
  const dave = await login("dave@globex.test");

  const orgs = (await api("/orgs", alice)).body.data;
  must(orgs.length === 2, "alice belongs to 2 organizations (multi-tenant seed data loaded)");
  const acme = orgs.find((o) => o.name === "Acme Corp").id;

  const dash = await api(`/orgs/${acme}/dashboard`, alice);
  must(dash.status === 200 && dash.body.data.total_projects === 3, "Acme dashboard shows the 3 seeded projects");

  must((await api(`/orgs/${acme}/dashboard`, dave)).status === 404, "another tenant's user gets 404 on Acme's dashboard");
  must((await api(`/orgs/${acme}/tasks`, dave)).status === 404, "another tenant's user gets 404 on Acme's tasks");
  must((await api("/orgs", null)).status === 401, "requests without a token are rejected (401)");
  console.log("\nsmoke test passed");
} catch (err) {
  console.error(`\n${err.message}`);
  process.exit(1);
}
