#!/usr/bin/env python3
"""
Memory-graph growth replay for the durable AKG agent.

Reconstructs, hand by hand, how the model-maintained agent populates its AKG
store: what it observed, what it reasoned, and the exact akg_put_node /
akg_put_edge writes it issued. Drives two demo artifacts from one parse of the
agent's post-hand write trail (update-session.jsonl):

  A. a terminal scrubber  (default)   — step through hands in the terminal
  B. a self-contained HTML graph      (--html OUT)  — scrub a growing node/edge
     graph with a hand slider; vanilla JS, no dependencies, opens offline.

Usage:
  python3 replay.py <agent-dir | update-session.jsonl> [--html OUT.html]
                    [--auto SECONDS] [--reconcile] [--from N] [--to N]

Examples:
  # terminal, step on Enter
  python3 replay.py research/experiments/phase2-fullhistory-vs-akg/sessions/\
fullhistory-vs-akg-1/agents/llm-akg-durable

  # auto-advance 1.5s/hand
  python3 replay.py <agent-dir> --auto 1.5

  # emit the HTML graph and verify the replayed end-state matches the real store
  python3 replay.py <agent-dir> --html docs/research/results/graph-growth.html --reconcile
"""

import argparse
import json
import os
import re
import sys
import time

# ----- ANSI (terminal renderer) ------------------------------------------------

class C:
    reset = "\033[0m"
    dim = "\033[2m"
    bold = "\033[1m"
    green = "\033[32m"
    yellow = "\033[33m"
    cyan = "\033[36m"
    magenta = "\033[35m"
    blue = "\033[34m"
    red = "\033[31m"


def _supports_color():
    return sys.stdout.isatty() and os.environ.get("TERM") not in (None, "dumb")


# ----- parsing -----------------------------------------------------------------

HAND_RE = re.compile(r"hand=(\d+)")
HAND_LINE_RE = re.compile(r"^hand=\d+ .*", re.MULTILINE)


def _resolve_jsonl(path):
    if os.path.isdir(path):
        cand = os.path.join(path, "update-session.jsonl")
        if not os.path.exists(cand):
            sys.exit(f"no update-session.jsonl in {path}")
        return cand
    return path


def _load_messages(jsonl_path):
    msgs = []
    with open(jsonl_path) as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                o = json.loads(line)
            except json.JSONDecodeError:
                # tolerate a partial trailing line while a session is still writing
                continue
            if o.get("type") == "message":
                msgs.append(o["message"])
    return msgs


def _observation_from_user(text):
    """Pull the 'hand that just finished' line and its hand number."""
    m = HAND_LINE_RE.search(text)
    line = m.group(0).strip() if m else ""
    hm = HAND_RE.search(line) or HAND_RE.search(text)
    hand = int(hm.group(1)) if hm else None
    return hand, line


def build_frames(jsonl_path):
    """Return (frames, final_graph).

    Each frame is one hand: {hand, observation, thinking, reads, mutations,
    node_count, edge_count, nodes_by_type, opp_body}. Mutations carry new/updated
    flags computed against the running graph state, so the renderer can highlight
    exactly what changed this hand.
    """
    msgs = _load_messages(jsonl_path)

    nodes = {}   # (type,id) -> {title, body, tags}
    edges = {}   # (ft,fi,rel,tt,ti) -> {strength}

    frames = []
    cur = None

    def nodes_by_type():
        out = {}
        for (t, _i) in nodes:
            out[t] = out.get(t, 0) + 1
        return out

    def flush():
        if cur is None:
            return
        cur["node_count"] = len(nodes)
        cur["edge_count"] = len(edges)
        cur["nodes_by_type"] = nodes_by_type()
        opp = nodes.get(("opponent", "villain"))
        cur["opp_body"] = opp["body"] if opp else ""
        frames.append(cur)

    for m in msgs:
        role = m.get("role")
        content = m.get("content", [])

        if role == "user":
            text = " ".join(c.get("text", "") for c in content if c.get("type") == "text")
            if "hand=" not in text:
                continue
            hand, obs = _observation_from_user(text)
            flush()
            cur = {
                "hand": hand,
                "observation": obs,
                "thinking": "",
                "reads": [],
                "mutations": [],
            }

        elif role == "assistant" and cur is not None:
            for c in content:
                if c.get("type") == "thinking":
                    cur["thinking"] += c.get("thinking", "")
                elif c.get("type") == "toolCall":
                    name = c.get("name")
                    args = c.get("arguments", {})
                    if name == "akg_get_node":
                        cur["reads"].append({"type": args.get("type"), "id": args.get("id")})
                    elif name == "akg_put_node":
                        key = (args.get("type"), args.get("id"))
                        is_new = key not in nodes
                        nodes[key] = {
                            "title": args.get("title", ""),
                            "body": args.get("body", ""),
                            "tags": args.get("tags", []),
                        }
                        cur["mutations"].append({
                            "kind": "node",
                            "new": is_new,
                            "type": args.get("type"),
                            "id": args.get("id"),
                            "title": args.get("title", ""),
                            "body": args.get("body", ""),
                        })
                    elif name == "akg_put_edge":
                        key = (args.get("from_type"), args.get("from_id"),
                               args.get("relation"),
                               args.get("to_type"), args.get("to_id"))
                        prev = edges.get(key, {}).get("strength")
                        strength = args.get("strength")
                        is_new = key not in edges
                        edges[key] = {"strength": strength}
                        cur["mutations"].append({
                            "kind": "edge",
                            "new": is_new,
                            "from_type": args.get("from_type"),
                            "from_id": args.get("from_id"),
                            "relation": args.get("relation"),
                            "to_type": args.get("to_type"),
                            "to_id": args.get("to_id"),
                            "strength": strength,
                            "prev_strength": prev,
                        })

    flush()
    final_graph = {
        "nodes": [{"type": t, "id": i, **v} for (t, i), v in nodes.items()],
        "edges": [{"from_type": k[0], "from_id": k[1], "relation": k[2],
                   "to_type": k[3], "to_id": k[4], **v} for k, v in edges.items()],
    }
    return frames, final_graph


# ----- reconcile against the real store ---------------------------------------

def reconcile(final_graph, agent_dir):
    """Compare replayed end-state to the committed memory-export.json."""
    export = os.path.join(agent_dir, "memory-export.json")
    if not os.path.exists(export):
        return None
    d = json.load(open(export))
    real_nodes = {(n.get("type"), n.get("id")) for n in d.get("nodes", [])}
    rep_nodes = {(n["type"], n["id"]) for n in final_graph["nodes"]}
    real_edges = {(e.get("from_type"), e.get("from_id"), e.get("relation"),
                   e.get("to_type"), e.get("to_id")) for e in d.get("edges", [])}
    rep_edges = {(e["from_type"], e["from_id"], e["relation"],
                  e["to_type"], e["to_id"]) for e in final_graph["edges"]}
    return {
        "real_nodes": len(real_nodes), "replay_nodes": len(rep_nodes),
        "nodes_only_real": sorted(real_nodes - rep_nodes),
        "nodes_only_replay": sorted(rep_nodes - real_nodes),
        "real_edges": len(real_edges), "replay_edges": len(rep_edges),
        "edges_match": real_edges == rep_edges,
        "nodes_match": real_nodes == rep_nodes,
    }


# ----- A. terminal scrubber ----------------------------------------------------

def render_terminal(frames, auto=None):
    color = _supports_color()

    def c(s, col):
        return f"{col}{s}{C.reset}" if color else s

    for idx, fr in enumerate(frames):
        os.system("clear" if os.name != "nt" else "cls") if (color and auto is None) else None
        bar = "=" * 72
        print(c(bar, C.dim))
        print(c(f" HAND {fr['hand']}", C.bold + C.cyan) +
              c(f"   ({idx + 1}/{len(frames)})", C.dim))
        print(c(bar, C.dim))

        print(c("\n  observed", C.dim))
        print("   " + (fr["observation"] or "(none)"))

        if fr["thinking"]:
            think = fr["thinking"].strip().replace("\n", "\n   ")
            print(c("\n  reasoned", C.dim))
            print(c("   " + think, C.yellow))

        if fr["reads"]:
            r = ", ".join(f"{x['type']}/{x['id']}" for x in fr["reads"])
            print(c("\n  read", C.dim) + "  " + c(r, C.blue))

        if fr["mutations"]:
            print(c("\n  wrote", C.dim))
            for mut in fr["mutations"]:
                tag = "NEW" if mut["new"] else "upd"
                tagc = C.green if mut["new"] else C.magenta
                if mut["kind"] == "node":
                    line = f"   [{tag}] node  {mut['type']}/{mut['id']}"
                    if mut["title"]:
                        line += c(f"  “{mut['title']}”", C.dim)
                    print(c(f"   [{tag}]", tagc) + c(" node ", C.dim) +
                          c(f" {mut['type']}/{mut['id']}", C.cyan) +
                          (c(f'  "{mut["title"]}"', C.dim) if mut["title"] else ""))
                else:
                    s = mut["strength"]
                    sd = ""
                    if not mut["new"] and mut["prev_strength"] is not None and mut["prev_strength"] != s:
                        sd = c(f"  ({mut['prev_strength']}→{s})", C.dim)
                    elif s is not None:
                        sd = c(f"  @{s}", C.dim)
                    print(c(f"   [{tag}]", tagc) + c(" edge ", C.dim) +
                          c(f" {mut['from_type']}/{mut['from_id']}", C.cyan) +
                          c(f" -{mut['relation']}→ ", C.dim) +
                          c(f"{mut['to_type']}/{mut['to_id']}", C.cyan) + sd)

        nbt = ", ".join(f"{t}:{n}" for t, n in sorted(fr["nodes_by_type"].items()))
        print(c("\n  graph now  ", C.dim) +
              c(f"{fr['node_count']} nodes", C.bold + C.green) + c(" / ", C.dim) +
              c(f"{fr['edge_count']} edges", C.bold + C.green) +
              c(f"   [{nbt}]", C.dim))

        if auto is not None:
            time.sleep(auto)
        else:
            try:
                input(c("\n  → Enter for next hand (Ctrl-C to stop) ", C.dim))
            except (EOFError, KeyboardInterrupt):
                print()
                return


# ----- B. self-contained HTML graph -------------------------------------------

HTML_TEMPLATE = r"""<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/>
<title>AKG memory-graph growth — __TITLE__</title>
<style>
  :root{--bg:#0d1117;--panel:#161b22;--line:#30363d;--txt:#c9d1d9;--dim:#8b949e;
        --acc:#58a6ff;--new:#3fb950;--upd:#bc8cff;}
  *{box-sizing:border-box} html,body{height:100%}
  body{margin:0;background:var(--bg);color:var(--txt);
    font:14px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;overflow:hidden}
  .wrap{display:flex;height:100vh}
  .left{flex:1;position:relative;min-width:0}
  canvas{display:block;width:100%;height:100%}
  .right{width:400px;border-left:1px solid var(--line);background:var(--panel);
    padding:18px;overflow:auto}
  h1{font-size:15px;margin:0 0 4px;color:var(--acc)}
  .sub{color:var(--dim);font-size:12px;margin-bottom:14px}
  .controls{position:absolute;left:18px;right:18px;bottom:18px;background:rgba(13,17,23,.9);
    border:1px solid var(--line);border-radius:10px;padding:14px 16px;backdrop-filter:blur(4px)}
  .controls .row{display:flex;align-items:center;gap:14px}
  input[type=range]{flex:1;height:4px;accent-color:var(--acc);cursor:pointer}
  button{background:#21262d;color:var(--txt);border:1px solid var(--line);
    border-radius:7px;padding:7px 16px;cursor:pointer;font:inherit;min-width:96px}
  button:hover{border-color:var(--acc);color:#fff}
  .hand-ctr{font-weight:700;color:var(--acc);white-space:nowrap;font-size:15px}
  .counts{white-space:nowrap;color:var(--dim)} .counts b{color:var(--new)}
  .obs{color:var(--txt);white-space:pre-wrap;word-break:break-word;font-size:12.5px}
  .think{color:#e3b341;white-space:pre-wrap;margin:6px 0;font-size:12.5px;max-height:220px;overflow:auto}
  .muts{margin:6px 0 0} .mut{padding:3px 0;font-size:12.5px}
  .tag{display:inline-block;width:38px;font-size:11px;font-weight:800;letter-spacing:.5px}
  .tag.new{color:var(--new)} .tag.upd{color:var(--upd)}
  .lbl{color:var(--dim);text-transform:uppercase;font-size:10.5px;letter-spacing:1px;
    margin:16px 0 5px;border-top:1px solid var(--line);padding-top:12px}
  .lbl:first-of-type{border-top:none;padding-top:0}
  .body{color:var(--dim);white-space:pre-wrap;font-size:12px;border-left:2px solid var(--line);
    padding-left:10px;margin-top:4px;max-height:200px;overflow:auto}
  .legend{position:absolute;top:14px;left:18px;font-size:12px;color:var(--dim);
    background:rgba(13,17,23,.7);border:1px solid var(--line);border-radius:8px;padding:8px 12px;max-width:60%}
  .legend span{display:inline-flex;align-items:center;margin:2px 14px 2px 0}
  .dot{width:10px;height:10px;border-radius:50%;display:inline-block;margin-right:6px}
  .hint{position:absolute;top:14px;right:18px;font-size:11px;color:var(--dim)}
</style></head><body>
<div class="wrap">
  <div class="left">
    <div class="legend" id="legend"></div>
    <div class="hint">drag slider · hover a node</div>
    <canvas id="cv"></canvas>
    <div class="controls">
      <div class="row">
        <button id="play">▶ Play</button>
        <span class="hand-ctr" id="handctr"></span>
        <input type="range" id="slider" min="0" max="0" value="0"/>
        <span class="counts" id="counts"></span>
      </div>
    </div>
  </div>
  <div class="right">
    <h1>memory-graph growth</h1>
    <div class="sub">__TITLE__ · model-maintained AKG store</div>
    <div class="lbl">observed</div>
    <div class="obs" id="obs"></div>
    <div class="lbl">reasoned</div>
    <div class="think" id="think"></div>
    <div class="lbl">wrote this hand</div>
    <div class="muts" id="muts"></div>
    <div class="lbl">opponent profile (prose body)</div>
    <div class="body" id="oppbody"></div>
  </div>
</div>
<script>
const FRAMES = __FRAMES_JSON__;

const cv = document.getElementById('cv'), ctx = cv.getContext('2d');
const slider = document.getElementById('slider');
const playBtn = document.getElementById('play');
slider.min = 0; slider.max = Math.max(FRAMES.length - 1, 0); slider.value = 0;

const PALETTE = ['#58a6ff','#3fb950','#bc8cff','#d29922','#f85149','#39c5cf','#db61a2','#a371f7','#ff7b72','#7ee787'];
const typeColor = {};
function colorFor(t){ if(!(t in typeColor)) typeColor[t] = PALETTE[Object.keys(typeColor).length % PALETTE.length]; return typeColor[t]; }

// ---- replay cumulative state + per-hand "added" sets ----
const states = [];
(function(){
  const nodes = new Map(), edges = new Map();
  for(const fr of FRAMES){
    const addedN = new Set(), addedE = new Set();
    for(const m of fr.mutations){
      if(m.kind === 'node'){
        const k = m.type + '/' + m.id;
        if(!nodes.has(k)) nodes.set(k, {key:k, type:m.type, id:m.id, title:m.title});
        else nodes.get(k).title = m.title;
        if(m.new) addedN.add(k);
      } else {
        const k = m.from_type+'/'+m.from_id+'|'+m.relation+'|'+m.to_type+'/'+m.to_id;
        edges.set(k, {key:k, a:m.from_type+'/'+m.from_id, b:m.to_type+'/'+m.to_id, rel:m.relation, s:m.strength});
        if(m.new) addedE.add(k);
      }
    }
    states.push({nodes:[...nodes.values()], edges:[...edges.values()], addedN, addedE, n:nodes.size, e:edges.size});
  }
})();
const finalState = states[states.length - 1];

// ---- degree-based sizing ----
const degree = {};
for(const n of finalState.nodes) degree[n.key] = 0;
for(const e of finalState.edges){ degree[e.a] = (degree[e.a]||0)+1; degree[e.b] = (degree[e.b]||0)+1; }
// node size is in SCREEN pixels and intentionally independent of camera zoom,
// so nodes stay readable whether the camera is framing 2 nodes or 60.
function radius(key){ return Math.max(13, Math.min(42, 13 + (degree[key]||0) * 1.9)); }

// ---- deterministic hub-centered spiral by appearance order ----
// The opponent node is created first and is the hub, so it lands at the center;
// every later node spirals outward in creation order. Stable positions (no jumps)
// and the graph visibly grows outward as hands accrue, so fit-to-visible zooms
// from tight (early) to wide (late) — the "watch it populate" effect.
const order = {}; let _oi = 0;
for(const fr of FRAMES) for(const m of fr.mutations)
  if(m.kind === 'node'){ const k = m.type+'/'+m.id; if(!(k in order)) order[k] = _oi++; }
const pos = {};
for(const n of finalState.nodes){ const o = order[n.key] || 0;
  const ang = o * 2.399963, r = o === 0 ? 0 : 70 + 38*Math.sqrt(o);
  pos[n.key] = {x:Math.cos(ang)*r, y:Math.sin(ang)*r}; }

// ---- camera (smooth fit-to-visible) ----
let cam = {x:0, y:0, scale:1}, target = {x:0, y:0, scale:1};
function computeTarget(st){
  const W = cv.clientWidth, H = cv.clientHeight;
  if(!st.nodes.length){ target = {x:0, y:0, scale:1}; return; }
  let minx=1e9, miny=1e9, maxx=-1e9, maxy=-1e9;
  for(const n of st.nodes){ const p = pos[n.key];
    minx=Math.min(minx,p.x); miny=Math.min(miny,p.y); maxx=Math.max(maxx,p.x); maxy=Math.max(maxy,p.y); }
  const pad = 120, w = Math.max(maxx-minx, 1), h = Math.max(maxy-miny, 1);
  let scale = Math.min((W-pad*2)/w, (H-pad*2)/h);
  scale = Math.max(0.45, Math.min(scale, 1.8));
  target = {x:(minx+maxx)/2, y:(miny+maxy)/2, scale};
}
function worldToScreen(p){ const W=cv.clientWidth, H=cv.clientHeight;
  return {x:(p.x-cam.x)*cam.scale + W/2, y:(p.y-cam.y)*cam.scale + H/2}; }

function fit(){ cv.width = cv.clientWidth*devicePixelRatio; cv.height = cv.clientHeight*devicePixelRatio;
  ctx.setTransform(devicePixelRatio,0,0,devicePixelRatio,0,0); }
window.addEventListener('resize', () => { fit(); computeTarget(states[curIndex]); });

// ---- index / interaction ----
let curIndex = 0;
function setIndex(i){ curIndex = i; slider.value = i; computeTarget(states[i]); renderPanel(i, states[i]); }

let hover = null;
cv.addEventListener('mousemove', ev => {
  const rect = cv.getBoundingClientRect(), mx = ev.clientX-rect.left, my = ev.clientY-rect.top;
  const st = states[curIndex]; hover = null;
  for(const n of st.nodes){ const s = worldToScreen(pos[n.key]), r = radius(n.key);
    if((mx-s.x)**2 + (my-s.y)**2 < (r+5)**2){ hover = {n, mx, my}; break; } }
});
cv.addEventListener('mouseleave', () => hover = null);

// ---- play ----
let playing = false, lastStep = 0;
const STEP_MS = 1100;
function stop(){ playing = false; playBtn.textContent = '▶ Play'; }
function start(){ if(curIndex >= states.length-1) setIndex(0);  // restart if at the end
  playing = true; playBtn.textContent = '❚❚ Pause'; lastStep = performance.now(); }
playBtn.addEventListener('click', () => { playing ? stop() : start(); });
// any interaction with the slider hard-stops playback (covers input/change/pointer)
function scrub(){ stop(); setIndex(+slider.value); }
slider.addEventListener('input', scrub);
slider.addEventListener('change', scrub);
slider.addEventListener('pointerdown', stop);

// ---- render loop (camera lerp + play stepping + draw) ----
function frame(ts){
  if(playing && ts - lastStep >= STEP_MS){
    lastStep = ts;
    if(curIndex >= states.length-1) stop();      // stop at the last hand; never wrap to 1
    else setIndex(curIndex + 1);
  }
  cam.x += (target.x-cam.x)*0.14; cam.y += (target.y-cam.y)*0.14; cam.scale += (target.scale-cam.scale)*0.14;

  const W = cv.clientWidth, H = cv.clientHeight;
  ctx.clearRect(0,0,W,H);
  const st = states[curIndex], shown = new Set(st.nodes.map(n=>n.key)), t = ts;

  for(const e of st.edges){ if(!shown.has(e.a)||!shown.has(e.b)) continue;
    const p = worldToScreen(pos[e.a]), q = worldToScreen(pos[e.b]), isNew = st.addedE.has(e.key);
    ctx.strokeStyle = isNew ? '#3fb950' : 'rgba(139,148,158,0.30)';
    ctx.lineWidth = isNew ? 2.4 : 1.1;
    ctx.beginPath(); ctx.moveTo(p.x,p.y); ctx.lineTo(q.x,q.y); ctx.stroke();
  }
  for(const nd of st.nodes){ const sp = worldToScreen(pos[nd.key]); let r = radius(nd.key);
    const isNew = st.addedN.has(nd.key);
    if(isNew){ const pulse = 1 + 0.05*Math.sin(t/220); r *= pulse;
      ctx.beginPath(); ctx.arc(sp.x,sp.y,r+4,0,7); ctx.fillStyle = 'rgba(63,185,80,0.10)'; ctx.fill(); }
    ctx.beginPath(); ctx.arc(sp.x,sp.y,r,0,7); ctx.fillStyle = colorFor(nd.type); ctx.fill();
    ctx.lineWidth = isNew ? 3 : 1.5; ctx.strokeStyle = isNew ? '#3fb950' : 'rgba(13,17,23,0.85)'; ctx.stroke();
  }
  ctx.font = '12px ui-monospace, monospace'; ctx.textBaseline = 'middle';
  for(const nd of st.nodes){ const sp = worldToScreen(pos[nd.key]), r = radius(nd.key);
    const hot = hover && hover.n.key === nd.key;
    if(st.nodes.length > 24 && (degree[nd.key]||0) < 2 && !hot) continue;
    const label = nd.id, tw = ctx.measureText(label).width;
    ctx.fillStyle = 'rgba(13,17,23,0.62)'; ctx.fillRect(sp.x+r+4, sp.y-9, tw+7, 18);
    ctx.fillStyle = '#e6edf3'; ctx.fillText(label, sp.x+r+7, sp.y+1);
  }
  if(hover){ const n = hover.n, lines = [n.type+'/'+n.id]; if(n.title) lines.push(n.title);
    ctx.font = '12px ui-monospace, monospace';
    let w = 0; for(const l of lines) w = Math.max(w, ctx.measureText(l).width);
    const bx = Math.min(hover.mx+14, W-w-26), by = hover.my+14, bw = w+18, bh = lines.length*18+12;
    ctx.fillStyle = 'rgba(22,27,34,0.98)'; ctx.strokeStyle = '#30363d'; ctx.lineWidth = 1;
    ctx.fillRect(bx,by,bw,bh); ctx.strokeRect(bx,by,bw,bh);
    lines.forEach((l,i) => { ctx.fillStyle = i ? '#8b949e' : '#e6edf3'; ctx.fillText(l, bx+9, by+16+i*18); });
  }
  requestAnimationFrame(frame);
}

// ---- side panel ----
function esc(s){ return (s||'').replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
function renderPanel(i, st){
  const fr = FRAMES[i];
  document.getElementById('handctr').textContent = 'hand ' + (fr.hand!=null?fr.hand:'?') + ' / ' + FRAMES.length;
  document.getElementById('obs').textContent = fr.observation || '(none)';
  document.getElementById('think').textContent = (fr.thinking || '—').trim();
  document.getElementById('counts').innerHTML = '<b>'+st.n+'</b> nodes / <b>'+st.e+'</b> edges';
  document.getElementById('oppbody').textContent = fr.opp_body || '—';
  const muts = fr.mutations.map(m => {
    const tag = m.new ? '<span class="tag new">NEW</span>' : '<span class="tag upd">UPD</span>';
    if(m.kind === 'node') return '<div class="mut">'+tag+' node <b>'+esc(m.type)+'/'+esc(m.id)+'</b></div>';
    let s = m.strength!=null ? ' @'+m.strength : '';
    if(!m.new && m.prev_strength!=null && m.prev_strength!==m.strength) s = ' ('+m.prev_strength+'→'+m.strength+')';
    return '<div class="mut">'+tag+' edge '+esc(m.from_id)+' <span style="color:var(--dim)">-'+esc(m.relation)+'→</span> '+esc(m.to_id)+s+'</div>';
  }).join('') || '<div class="mut" style="color:var(--dim)">no writes</div>';
  document.getElementById('muts').innerHTML = muts;
}
function renderLegend(){
  document.getElementById('legend').innerHTML = Object.entries(typeColor)
    .map(([t,c]) => '<span><i class="dot" style="background:'+c+'"></i>'+t+'</span>').join('');
}

// ---- init ----
finalState.nodes.forEach(n => colorFor(n.type));   // assign colors deterministically by appearance
renderLegend();
fit(); computeTarget(states[0]); cam = {x:target.x, y:target.y, scale:target.scale}; setIndex(0);
requestAnimationFrame(frame);
</script></body></html>
"""


def render_html(frames, out_path, title):
    payload = json.dumps(frames, separators=(",", ":"))
    html = HTML_TEMPLATE.replace("__FRAMES_JSON__", payload).replace("__TITLE__", title)
    os.makedirs(os.path.dirname(os.path.abspath(out_path)), exist_ok=True)
    with open(out_path, "w") as f:
        f.write(html)
    return out_path


# ----- main --------------------------------------------------------------------

def main():
    ap = argparse.ArgumentParser(description="Replay AKG memory-graph growth.")
    ap.add_argument("path", help="agent dir or update-session.jsonl")
    ap.add_argument("--html", metavar="OUT", help="write self-contained HTML graph")
    ap.add_argument("--auto", type=float, metavar="SEC",
                    help="terminal: auto-advance SEC per hand (default: step on Enter)")
    ap.add_argument("--reconcile", action="store_true",
                    help="verify replayed end-state vs memory-export.json")
    ap.add_argument("--from", dest="frm", type=int, help="start at hand N")
    ap.add_argument("--to", dest="to", type=int, help="stop at hand N")
    args = ap.parse_args()

    jsonl = _resolve_jsonl(args.path)
    agent_dir = os.path.dirname(jsonl)
    title = os.path.basename(os.path.dirname(agent_dir)) + "/" + os.path.basename(agent_dir)

    frames, final_graph = build_frames(jsonl)
    if not frames:
        sys.exit("no hand frames found (is this a durable-agent update-session.jsonl?)")

    if args.frm is not None:
        frames = [f for f in frames if f["hand"] is None or f["hand"] >= args.frm]
    if args.to is not None:
        frames = [f for f in frames if f["hand"] is None or f["hand"] <= args.to]

    if args.reconcile:
        rec = reconcile(final_graph, agent_dir)
        if rec is None:
            print("reconcile: no memory-export.json (session may still be running)\n")
        else:
            ok = rec["nodes_match"] and rec["edges_match"]
            print(f"reconcile: replay {rec['replay_nodes']}n/{rec['replay_edges']}e "
                  f"vs store {rec['real_nodes']}n/{rec['real_edges']}e  "
                  f"-> {'MATCH' if ok else 'MISMATCH'}")
            if rec["nodes_only_real"]:
                print("  only in store:", rec["nodes_only_real"][:10])
            if rec["nodes_only_replay"]:
                print("  only in replay:", rec["nodes_only_replay"][:10])
            print()

    if args.html:
        p = render_html(frames, args.html, title)
        print(f"wrote {p}  ({len(frames)} hands, "
              f"{final_graph['nodes'].__len__()} nodes / {final_graph['edges'].__len__()} edges)")
        if not args.auto and sys.stdout.isatty():
            return  # producing HTML; don't also block on the terminal scrubber
    if not args.html or args.auto is not None:
        render_terminal(frames, auto=args.auto)


if __name__ == "__main__":
    main()
