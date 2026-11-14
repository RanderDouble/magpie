// The saved ChatGPT accounts standing in while Codex is signed out of a
// ChatGPT sign-in (codexStandIn, Provider.NoSavedStandIn): the tick under
// Providers › Codex's account switches. app.js imports this when it draws
// Codex's accounts, so the core doesn't carry it.

const SI_AGENT = { agent: "Codex" };
const siLabel = () => t("Use the saved {agent} accounts while it is signed out", SI_AGENT);
const siTitle = () => t("On: while {agent} has no ChatGPT sign-in of its own (an API key in auth.json), a saved ChatGPT account stands in for it, and the accounts ticked are used beside it, as when {agent} is signed in. Off: {agent} is listed only while it is signed in itself.", SI_AGENT);

// savedStandInLink appends the way back on to a Codex line that says its
// saved accounts aren't offered: the tick is drawn with Codex's accounts,
// and the stand-in off leaves those out of the list.
window.savedStandInLink = function (x, r) {
  const use = el("button", "link", t("Use the saved {agent} accounts while it is signed out", { agent: x.agentName }));
  use.title = siTitle();
  use.onclick = () => accountAction("provider/savedstandin", { id: x.agent, noSavedStandIn: false }, t("Saved"));
  const line = r.lastChild;
  line.append(line.children.length ? " · " : " ", use);
};

// savedStandInRow puts the tick in list, for p's Codex, right under the
// restart row: app.js draws its plus rows before this file is in, so the
// tick is placed against the rows already there rather than appended after
// them
window.savedStandInRow = function (p, list) {
  const [si, cb] = tick(siLabel(), !p.noSavedStandIn);
  si.title = siTitle();
  cb.onchange = () => accountAction("provider/savedstandin", { id: p.id, noSavedStandIn: !cb.checked }, t("Saved"));
  const row = el("div", "saved-standin", si);
  const above = list.querySelector(".restart-app") || list.querySelector(".switch-only") || list.querySelector(".auto-switch");
  if (above) above.after(row);
  else {
    const add = list.querySelector(".acc.add");
    add ? list.insertBefore(row, add) : list.append(row);
  }
};
