"""Merge the three runs (first non-error result per line) and score tier rules.

Run from the repo root: python .planning/spikes/001-paste-confidence-tiers/analyze.py
"""
import json, pathlib

d = pathlib.Path(__file__).parent
runs = [json.loads((d / f"rps{r}.json").read_text(encoding="utf-8")) for r in ("1.0", "0.8", "0.5")]

rows = []
for i, r0 in enumerate(runs[0]):
    pick = next((run[i] for run in runs if not run[i].get("error")), r0)
    rows.append(pick)

def base(r, high):
    if r.get("error"):
        return "Error"
    if not r["results"]:
        return "NotFound"
    return "High" if high else "Medium"

def exacts(r):
    return r.get("exacts") or []

def unique_at(min_score):
    return lambda r: base(r, sum(e["score"] >= min_score for e in exacts(r)) == 1)

rules = {
    "D02": ("unique exact among 10 results (current)", lambda r: base(r, len(exacts(r)) == 1)),
    "S100": ("unique exact among results scoring 100", unique_at(100)),
    "S90": ("unique exact among results scoring >= 90", unique_at(90)),
    "TOPANY": ("top result is exact", lambda r: base(r, bool(exacts(r)) and exacts(r)[0]["rank"] == 1)),
}

def high_pick(r, key):
    """The exact match a High row would pre-select under each rule."""
    ex = exacts(r)
    if key == "D02":
        return ex[0]
    if key == "TOPANY":
        return ex[0]
    m = {"S100": 100, "S90": 90}[key]
    return next(e for e in ex if e["score"] >= m)

out = ["| # | Line | " + " | ".join(rules) + " | High pick (rank:score · disambiguation) |",
       "|---|---|" + "---|" * len(rules) + "---|"]
for i, r in enumerate(rows, 1):
    tiers = {k: fn(r) for k, (_, fn) in rules.items()}
    pick = ""
    if tiers["S90"] == "High":
        e = high_pick(r, "S90")
        pick = f'{e["rank"]}:{e["score"]} · {e["disambiguation"] or "(none)"}'
    elif r.get("results"):
        t = r["results"][0]
        pick = f'top was {t["name"]} ({t["score"]})'
    out.append(f'| {i} | {r["line"]} | ' + " | ".join(tiers.values()) + f" | {pick.replace('|', '/')} |")

out += ["", "| Rule | Meaning | High | Medium | Not found | Error |", "|---|---|---|---|---|---|"]
for k, (desc, fn) in rules.items():
    c = {}
    for r in rows:
        t = fn(r)
        c[t] = c.get(t, 0) + 1
    h = c.get("High", 0)
    out.append(f'| {k} | {desc} | {h} ({100 * h // len(rows)}%) | {c.get("Medium", 0)} | {c.get("NotFound", 0)} | {c.get("Error", 0)} |')

errs = {r: sum(1 for x in run if x.get("error")) for r, run in zip(("1.0", "0.8", "0.5"), runs)}
out += ["", "| Run (req/s) | 503 errors of 50 |", "|---|---|"] + [f"| {r} | {n} |" for r, n in errs.items()]

text = "\n".join(out) + "\n"
(d / "merged.md").write_text(text, encoding="utf-8")
print(text)
