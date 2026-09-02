/** Product copy kept in one place so wording stays consistent across views. */

export const auth = {
  intro:
    "koa lists every repository tagged koa that you own or can reach through an organization, so signing in is what makes Discover useful. The Device Flow shows a short code you approve on github.com — no password or client secret is involved.",
  manualCaveat:
    "A hand-scoped token works too. GitHub's org-membership endpoint returns an empty list for fine-grained tokens by design, so on that path you name the organizations koa should search yourself, in Settings.",
  scopes:
    "Device Flow requests repo, to read private repositories and their release assets, and read:org, only to learn which organizations to search. The token is stored in your OS credential store.",
};
