/* ─── State ─── */
const S = { tokens: [], keys: [], cache: [] };

/* ─── Formatters & Escape ─── */
function esc(v) {
  return String(v ?? '').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
}
function fmtTime(v) {
  if (!v) return '—';
  const d = new Date(v);
  return isNaN(d) ? '—' : d.toLocaleString();
}
function fmtBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB'], i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}
function fmtNum(n) {
  return n ? new Intl.NumberFormat().format(n) : '0';
}
function iMore() { return `<svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/></svg>`; }
function iCopy() { return `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`; }

/* ─── Core Request ─── */
async function req(path, opts = {}) {
  const r = await fetch(path, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) }, ...opts });
  const ct = r.headers.get('content-type') || '';
  const body = ct.includes('application/json') ? await r.json().catch(() => null) : await r.text();
  if (!r.ok) throw new Error(body?.error?.message || body?.message || (typeof body === 'string' && body) || 'Request failed');
  return body;
}

/* ─── Toast & Flash ─── */
const toastEl = document.getElementById('toast');
let _toastTimer;
function toast(msg, kind = 'ok') {
  clearTimeout(_toastTimer);
  toastEl.innerHTML = kind === 'ok' 
    ? `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--accent-green)" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>` 
    : `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--accent-red)" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`;
  toastEl.innerHTML += ` <span>${esc(msg)}</span>`;
  toastEl.className = 'show ' + kind;
  _toastTimer = setTimeout(() => { toastEl.className = ''; }, 3500);
}

function flash(msg, kind = 'error', rawVal = '', target = 'flashApp') {
  const f = document.getElementById(target);
  f.className = kind;
  if (rawVal) {
    f.innerHTML = `<strong>${esc(msg)}</strong><div class="fkey">${esc(rawVal)}</div><button class="ghost" onclick="clip('${esc(rawVal)}')">${iCopy()} Copy Key</button>`;
  } else {
    f.innerHTML = `<strong>${esc(msg)}</strong>`;
  }
  f.classList.remove('hidden');
}
function clearFlash() {
  document.getElementById('flashApp').classList.add('hidden');
  document.getElementById('flashLogin').classList.add('hidden');
}

async function clip(text) {
  try { await navigator.clipboard.writeText(text); toast('Copied to clipboard'); }
  catch { toast('Copy failed', 'err'); }
}

/* ─── Modal ─── */
const modalOv = document.getElementById('modalOv');
let _mCb = null;
function openModal({ title, msg = '', fields = [], confirmTxt = 'Confirm', confirmCls = '', onConfirm }) {
  document.getElementById('mTitle').textContent = title;
  document.getElementById('mMsg').textContent = msg;
  const fieldsEl = document.getElementById('mFields');
  fieldsEl.innerHTML = '';
  for (const f of fields) {
    const lbl = document.createElement('label');
    lbl.textContent = f.label;
    const el = f.type === 'textarea' 
      ? Object.assign(document.createElement('textarea'), { rows: 3 })
      : Object.assign(document.createElement('input'), { type: f.type || 'text' });
    el.id = 'mf_' + f.key;
    el.value = f.value ?? '';
    if (f.placeholder) el.placeholder = f.placeholder;
    if (f.readonly) {
      el.readOnly = true;
      el.style.fontFamily = 'var(--font-mono)';
      el.style.fontSize = '12px';
      const wrapper = document.createElement('div');
      wrapper.style.cssText = 'display:flex;gap:8px;align-items:center;';
      const copyBtn = document.createElement('button');
      copyBtn.type = 'button';
      copyBtn.className = 'ghost';
      copyBtn.style.cssText = 'flex-shrink:0;padding:8px 14px;';
      copyBtn.textContent = 'Copy';
      copyBtn.addEventListener('click', () => clip(el.value));
      wrapper.appendChild(el);
      wrapper.appendChild(copyBtn);
      lbl.appendChild(wrapper);
    } else {
      lbl.appendChild(el);
    }
    fieldsEl.appendChild(lbl);
  }
  const btn = document.getElementById('mConfirm');
  btn.textContent = confirmTxt;
  btn.className = 'primary ' + confirmCls;
  _mCb = onConfirm;
  modalOv.classList.remove('hidden');
  requestAnimationFrame(() => modalOv.classList.add('show'));
  setTimeout(() => (fieldsEl.querySelector('input,textarea') || btn).focus(), 80);
}
function closeModal() {
  modalOv.classList.remove('show');
  setTimeout(() => modalOv.classList.add('hidden'), 200);
  _mCb = null;
}
document.getElementById('mCancel').addEventListener('click', closeModal);
document.getElementById('mConfirm').addEventListener('click', () => {
  if (!_mCb) return closeModal();
  const vals = {};
  document.getElementById('mFields').querySelectorAll('input,textarea').forEach(el => vals[el.id.replace('mf_', '')] = el.value);
  _mCb(vals);
  closeModal();
});
modalOv.addEventListener('click', e => { if (e.target === modalOv) closeModal(); });

/* ─── Dropdowns ─── */
let _activeDd = null;
function toggleDd(id) {
  const el = document.getElementById(id);
  if (_activeDd && _activeDd !== el) _activeDd.classList.add('hidden');
  el.classList.toggle('hidden');
  _activeDd = el.classList.contains('hidden') ? null : el;
}
document.addEventListener('click', e => {
  if (_activeDd && !e.target.closest('.dropdown')) { _activeDd.classList.add('hidden'); _activeDd = null; }
});

/* ─── View / Routing ─── */
const views = ['overview', 'tokens', 'apikeys', 'usage', 'config', 'cache'];
async function switchView(target) {
  if (!views.includes(target)) target = 'overview';
  
  // Update sidebar UI
  document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
  document.querySelector(`.nav-item[data-view="${target}"]`)?.classList.add('active');

  // Smooth fade
  const pc = document.getElementById('pageContainer');
  pc.classList.add('fade-out');
  
  setTimeout(async () => {
    views.forEach(v => document.getElementById('view_' + v).classList.add('hidden'));
    document.getElementById('view_' + target).classList.remove('hidden');
    clearFlash();
    
    try {
      if (target === 'overview') await refreshOverview();
      else if (target === 'tokens') await refreshTokens();
      else if (target === 'apikeys') await refreshKeys();
      else if (target === 'usage') await refreshUsage();
      else if (target === 'config') await refreshConfig();
      else if (target === 'cache') await refreshCache();
    } catch (err) {
      toast('Failed loading data: ' + err.message, 'err');
    }
    
    pc.classList.remove('fade-out');
  }, 200);
}

window.addEventListener('hashchange', () => {
  const hash = window.location.hash.substring(1);
  switchView(hash || 'overview');
});

/* ─── Helpers Components ─── */
function bc(v) { return 'badge ' + String(v || '').replace(/[^a-zA-Z0-9_-]/g, ''); }
function qBar(cur, tot) {
  if (!tot) return `<div class="q-bar-wrapper"><div class="q-bar-text"><span>${cur}</span><span>—</span></div></div>`;
  const pct = Math.max(0, Math.min(100, Math.round(cur / tot * 100)));
  const cls = pct >= 80 ? 'good' : pct >= 40 ? 'warn' : 'danger';
  return `<div class="q-bar-wrapper"><div class="q-bar-text"><span>${fmtNum(cur)}</span><span>${fmtNum(tot)}</span></div><div class="q-bar-bg"><div class="q-bar-fill ${cls}" style="width:${pct}%"></div></div></div>`;
}

/* ─── LOADERS ─── */
async function refreshOverview() {
  const s = await req('/admin/system/status');
  document.getElementById('st_tok_tot').textContent = fmtNum(s.tokens.total);
  document.getElementById('st_tok_act').textContent = s.tokens.active + ' active tokens';
  document.getElementById('st_key_tot').textContent = fmtNum(s.api_keys.total);
  document.getElementById('st_key_act').textContent = s.api_keys.active + ' active keys';
  
  const dbc = s.config?.db_override_count ?? 0;
  document.getElementById('st_cfg_src').innerHTML = `<span class="badge ${s.config?.app_key_source === 'DB' ? 'active' : 'warning'}">${esc(s.config?.app_key_source)}</span>`;
  document.getElementById('st_cfg_db').textContent = dbc + ' DB overrides active';

  try {
    const q = await req('/admin/stats/quota');
    // API returns { pools: [{pool, total_chat_quota, remaining_chat_quota, ...}] }
    const pools = q.pools || [];
    let totalQuota = 0, remainingQuota = 0;
    pools.forEach(p => { totalQuota += (p.total_chat_quota || 0); remainingQuota += (p.remaining_chat_quota || 0); });
    const usedQuota = totalQuota - remainingQuota;
    document.getElementById('st_quota').textContent = fmtNum(usedQuota) + ' / ' + fmtNum(totalQuota);
    const pct = totalQuota > 0 ? Math.round(remainingQuota / totalQuota * 100) : 0;
    document.getElementById('st_quota_usage').textContent = 'Remaining / Total (' + pct + '% remaining)';
  } catch {
    document.getElementById('st_quota').textContent = 'ERROR';
  }

  // Load recent usage
  try {
    const u = await req('/admin/usage/logs?page_size=5');
    const tbody = document.getElementById('overviewRecentLogs');
    if (!u.data || u.data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="muted">No recent requests</td></tr>';
      return;
    }
    tbody.innerHTML = u.data.map(l => 
      `<tr>
        <td class="text-xs muted">${esc(fmtTime(l.created_at))}</td>
        <td>${esc(l.app_key_name || '—')}</td>
        <td><code>${esc(l.model || l.endpoint || '—')}</code></td>
        <td class="text-xs">${fmtNum(l.tokens_input || 0)}</td>
        <td class="text-xs">${fmtNum(l.tokens_output || 0)}</td>
        <td class="text-xs muted">${l.duration_ms}ms</td>
      </tr>`
    ).join('');
  } catch (e) {
    document.getElementById('overviewRecentLogs').innerHTML = '<tr><td colspan="6" class="muted text-danger">Failed to load logs</td></tr>';
  }
}

async function refreshTokens() {
  const res = await req('/admin/tokens?page_size=200');
  S.tokens = res.data || [];
  const tbody = document.getElementById('tokensTableBody');
  if (!S.tokens.length) { tbody.innerHTML = '<tr><td colspan="8" class="muted">No tokens found.</td></tr>'; return; }
  
  tbody.innerHTML = S.tokens.map(t => {
    const did = 'dd-t-' + t.id;
    const active = t.status === 'active';
    return `<tr>
      <td class="text-xs muted">#${t.id}</td>
      <td><code>${esc(t.token.substring(0,8))}...${esc(t.token.slice(-4))}</code></td>
      <td><span class="${bc(t.pool)}">${esc(t.pool || '—')}</span></td>
      <td><span class="${bc(t.status)}">${esc(t.status)}</span></td>
      <td>${qBar(t.chat_quota, t.total_chat_quota)}</td>
      <td class="text-xs muted">${esc(fmtTime(t.last_used))}</td>
      <td class="text-xs muted">${esc(t.remark || '—')}</td>
      <td>
        <div class="row-actions">
          <button class="ghost" onclick="doAction('tokens', ${t.id}, 'refresh')">Sync</button>
          <div class="dropdown">
            <button class="ghost" onclick="toggleDd('${did}')">${iMore()}</button>
            <div class="dropdown-menu hidden" id="${did}">
              <button onclick="doToggleToken(${t.id}, '${active ? 'disabled' : 'active'}'); toggleDd('${did}')">${active ? 'Disable' : 'Enable'}</button>
              <button onclick="doReplaceToken(${t.id}); toggleDd('${did}')">Replace Token</button>
              <div class="dd-divider"></div>
              <button class="danger" onclick="doDeleteToken(${t.id}); toggleDd('${did}')">Delete</button>
            </div>
          </div>
        </div>
      </td>
    </tr>`;
  }).join('');
}

async function refreshKeys() {
  const res = await req('/admin/apikeys?page_size=100');
  S.keys = res.data || [];
  const tbody = document.getElementById('keysTableBody');
  if (!S.keys.length) { tbody.innerHTML = '<tr><td colspan="8" class="muted">No API keys found.</td></tr>'; return; }

  tbody.innerHTML = S.keys.map(k => {
    const did = 'dd-k-' + k.id;
    const active = k.status === 'active';
    return `<tr>
      <td class="text-xs muted">#${k.id}</td>
      <td style="font-weight:500;">${esc(k.name)}</td>
      <td>
        <div class="flex items-center gap-2">
          <code style="max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${esc(k.key)}</code>
        </div>
      </td>
      <td><span class="${bc(k.status)}">${esc(k.status)}</span></td>
      <td class="text-xs" style="color:var(--text-secondary)">${k.rate_limit}/m · ${k.daily_limit}/d</td>
      <td class="text-xs"><span class="badge ${k.daily_used > k.daily_limit*0.8 ? 'warning' : ''}">${k.daily_used} today</span></td>
      <td class="text-xs muted">${esc(fmtTime(k.last_used_at))}</td>
      <td>
        <div class="row-actions">
          <button class="ghost" onclick="doToggleKey(${k.id}, '${active ? 'inactive' : 'active'}')">${active ? 'Disable' : 'Enable'}</button>
          <div class="dropdown">
            <button class="ghost" onclick="toggleDd('${did}')">${iMore()}</button>
            <div class="dropdown-menu hidden" id="${did}">
              <button onclick="doRegenKey(${k.id}); toggleDd('${did}')">Regenerate Key</button>
              <div class="dd-divider"></div>
              <button class="danger" onclick="doDeleteKey(${k.id}); toggleDd('${did}')">Delete</button>
            </div>
          </div>
        </div>
      </td>
    </tr>`;
  }).join('');
}

async function refreshUsage() {
  const p = document.getElementById('usagePeriod').value || '24h';
  
  // Total stats map to the system/usage endpoint
  try {
    const s = await req('/admin/system/usage?period=' + p);
    document.getElementById('usg_req').textContent = fmtNum(s.requests);
    document.getElementById('usg_tok').textContent = fmtNum((s.tokens_input || 0) + (s.tokens_output || 0));
    const errRate = s.requests > 0 ? ((s.errors / s.requests) * 100).toFixed(1) : '0';
    document.getElementById('usg_err').textContent = errRate + '%';
  } catch {
    document.getElementById('usg_req').textContent = 'ERR';
  }

  // Reload logs table
  try {
    const res = await req('/admin/usage/logs?page_size=20');
    const tbody = document.getElementById('usageLogsBody');
    if (!res.data || !res.data.length) { tbody.innerHTML = '<tr><td colspan="7" class="muted">No logs found.</td></tr>'; return; }
    
    tbody.innerHTML = res.data.map(l => 
      `<tr>
        <td class="text-xs muted">${esc(fmtTime(l.created_at))}</td>
        <td>${esc(l.app_key_name || '—')}</td>
        <td><code>${esc(l.model || '—')}</code></td>
        <td class="text-xs muted">${esc(l.endpoint || '—')}</td>
        <td class="text-xs"><span class="badge" title="Input">${fmtNum(l.tokens_input || 0)}</span> / <span class="badge" title="Output">${fmtNum(l.tokens_output || 0)}</span></td>
        <td class="text-xs muted">${l.duration_ms}ms</td>
        <td><span class="${l.status >= 400 ? 'badge error' : 'badge success'}">${l.status >= 400 ? 'ERR' : 'OK'}</span></td>
      </tr>`
    ).join('');
  } catch {}
}

async function refreshConfig() {
  const c = await req('/admin/config');
  document.getElementById('cfg_loglevel').value = c.log_level || 'info';
  document.getElementById('cfg_cf_timeout').value = c.proxy?.cloudflare_timeout || 60;
  document.getElementById('cfg_auth_retry').value = c.proxy?.max_auth_retries || 3;
  document.getElementById('cfg_tg_token').value = c.proxy?.telegram_bot_token || '';
  document.getElementById('cfg_tg_chat').value = c.proxy?.telegram_chat_id || '';
  document.getElementById('cfg_app_key').value = ''; // Don't show password
}

async function refreshCache() {
  try {
    const s = await req('/admin/cache/stats');
    const totCount = (s.image?.count || 0) + (s.video?.count || 0);
    const totSizeMB = (s.image?.size_mb || 0) + (s.video?.size_mb || 0);
    
    const cg = document.getElementById('cacheStatsGrid');
    cg.innerHTML = `
      <div class="stat-card"><div class="stat-label">Cache Storage</div><div class="stat-value">${fmtBytes(totSizeMB * 1048576)}</div><div class="stat-desc">${totCount} files stored</div></div>
      <div class="stat-card"><div class="stat-label">Cache Limit</div><div class="stat-value">Dynamic</div><div class="stat-desc">Managed by storage limits</div></div>
    `;
    
    const [resImg, resVid] = await Promise.all([
      req('/admin/cache/files?type=image&page_size=50').catch(() => ({ items: [] })),
      req('/admin/cache/files?type=video&page_size=50').catch(() => ({ items: [] }))
    ]);
    const files = [...(resImg.items || []), ...(resVid.items || [])].sort((a,b) => b.mod_time_ms - a.mod_time_ms);
    const tbody = document.getElementById('cacheFilesBody');
    if (!files.length) { tbody.innerHTML = '<tr><td colspan="5" class="muted">Cache is empty.</td></tr>'; return; }
    
    tbody.innerHTML = files.map(f => {
      const isVid = f.name.endsWith('.mp4');
      const type = isVid ? 'video' : 'image';
      return `
      <tr>
        <td><code>${esc(f.name)}</code></td>
        <td class="text-xs muted">${fmtBytes(f.size_bytes)}</td>
        <td class="text-xs"><span class="badge ${isVid?'free':'premium'}">${esc(type)}</span></td>
        <td class="text-xs muted">${esc(fmtTime(f.mod_time_ms))}</td>
        <td><button class="ghost danger-text" onclick="doDelCacheFile('${type}', '${esc(f.name)}')">Remove</button></td>
      </tr>
      `;
    }).join('');
  } catch (err) {
    document.getElementById('cacheFilesBody').innerHTML = `<tr><td colspan="5" class="muted text-danger">Failed: ${esc(err.message)}</td></tr>`;
  }
}


/* ─── ACTION HANDLERS ─── */
async function doAction(resource, id, action, method='POST', body=null) {
  try {
    await req(`/admin/${resource}/${id}/${action}`, { method, body: body ? JSON.stringify(body) : null });
    toast('Action successful');
    if (resource === 'tokens') refreshTokens();
    if (resource === 'apikeys') refreshKeys();
  } catch(e) { flash(e.message, 'error'); }
}

function doReplaceToken(id) {
  openModal({
    title: 'Replace Token #' + id,
    msg: 'Paste the new cookie bundle string from DevTools. Pool assignment will be re-evaluated.',
    fields: [{ key: 'token', label: 'New Cookie Bundle', type: 'textarea', placeholder: '_C_Auth=...; _EDGE_S=SID=...; MC1=GUID=...' }],
    onConfirm: async (v) => {
      try {
        await req(`/admin/tokens/${id}/replace`, { method: 'POST', body: JSON.stringify({ token: (v.token||'').trim(), reclassify: true }) });
        toast('Token replaced'); refreshTokens();
      } catch(e) { flash(e.message, 'error'); }
    }
  });
}

function doDeleteToken(id) {
  openModal({
    title: 'Delete Token', msg: 'Are you sure you want to permanently delete token #' + id + '?', confirmTxt: 'Delete', confirmCls: 'danger',
    onConfirm: async () => {
      try { await req(`/admin/tokens/${id}`, { method: 'DELETE' }); toast('Deleted'); refreshTokens(); } 
      catch(e) { flash(e.message, 'error'); }
    }
  });
}

async function doToggleToken(id, status) {
  try { await req(`/admin/tokens/${id}`, { method: 'PUT', body: JSON.stringify({ status }) }); toast('Token ' + status); refreshTokens(); }
  catch(e) { flash(e.message, 'error'); }
}

function doRegenKey(id) {
  openModal({
    title: 'Regenerate Key', msg: 'Connected clients will IMMEDIATELY lose access.', confirmTxt: 'Regenerate', confirmCls: 'danger',
    onConfirm: async () => {
      try {
        const r = await req(`/admin/apikeys/${id}/regenerate`, { method: 'POST' });
        openModal({
          title: 'Key Regenerated', 
          msg: 'Please update your clients with the new secret key. You will not be able to view it again.', 
          fields: [{ key: 'newKey', label: 'Secret Key', value: r.key, readonly: true }],
          confirmTxt: 'Close',
          onConfirm: () => {}
        });
        refreshKeys();
      } catch(e) { flash(e.message, 'error'); }
    }
  });
}

function doDeleteKey(id) {
  openModal({
    title: 'Delete Key', msg: 'Permanently revoke client API Key?', confirmTxt: 'Delete', confirmCls: 'danger',
    onConfirm: async () => {
      try { await req(`/admin/apikeys/${id}`, { method: 'DELETE' }); toast('Key Deleted'); refreshKeys(); } 
      catch(e) { flash(e.message, 'error'); }
    }
  });
}

async function doToggleKey(id, status) {
  try { await req(`/admin/apikeys/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) }); toast('Key ' + status); refreshKeys(); }
  catch(e) { flash(e.message, 'error'); }
}

/* Cache Handles */
function doClearCache() {
  openModal({
    title: 'Clear Entire Cache', msg: 'Are you sure? This will delete all cached resources immediately.', confirmTxt: 'Clear Cache', confirmCls: 'danger',
    onConfirm: async () => {
      try { 
        await req('/admin/cache/clear', { method: 'POST', body: JSON.stringify({ type: 'image' }) }); 
        await req('/admin/cache/clear', { method: 'POST', body: JSON.stringify({ type: 'video' }) }).catch(e => null); 
        toast('Cache cleared'); refreshCache(); 
      } catch(e) { flash(e.message, 'error'); }
    }
  });
}
async function doDelCacheFile(type, name) {
  try { await req('/admin/cache/delete', { method: 'POST', body: JSON.stringify({ type, names: [name] }) }); toast('File removed'); refreshCache(); }
  catch(e) { flash(e.message, 'error'); }
}


/* ─── FORMS ─── */
document.getElementById('loginForm').addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('button'); btn.disabled = true;
  try {
    await req('/admin/login', { method: 'POST', body: JSON.stringify({ key: document.getElementById('appKeyInput').value }) });
    document.getElementById('loginView').classList.add('hidden');
    document.getElementById('appView').classList.remove('hidden');
    toast('Logged in successfully');
    switchView(window.location.hash.substring(1) || 'overview');
  } catch (err) { flash(err.message, 'error', '', 'flashLogin'); }
  finally { btn.disabled = false; }
});

document.getElementById('logoutButton').addEventListener('click', async () => {
  try { await req('/admin/logout', { method: 'POST' }); } catch {}
  document.getElementById('loginView').classList.remove('hidden');
  document.getElementById('appView').classList.add('hidden');
  toast('Signed out');
});

document.getElementById('tokenImportForm').addEventListener('submit', async e => {
  e.preventDefault();
  const tokens = document.getElementById('tokenImportInput').value.split(/[\n,]+/).map(s=>s.trim()).filter(Boolean);
  if(!tokens.length) return flash('Enter at least one token', 'error');
  
  const pool = document.getElementById('tokenImportPool').value;
  const quota = document.getElementById('tokenImportQuota').value;
  const remark = document.getElementById('tokenImportRemark').value;
  
  const body = { operation: 'import', tokens, pool, remark };
  if(quota) body.quota = parseInt(quota, 10);
  
  const btn = document.getElementById('tokenImportBtn'); btn.disabled = true;
  try {
    const res = await req('/admin/tokens/batch', { method:'POST', body: JSON.stringify(body) });
    document.getElementById('tokenImportInput').value = '';
    toast(`Imported ${res.success} tokens` + (res.failed ? ` (${res.failed} failed)` : ''));
    refreshTokens();
  } catch(err) { flash(err.message, 'error'); }
  finally { btn.disabled = false; }
});

document.getElementById('keyCreateForm').addEventListener('submit', async e => {
  e.preventDefault();
  const name = document.getElementById('keyNameInput').value;
  const rate = parseInt(document.getElementById('keyRateInput').value, 10) || 60;
  const daily = parseInt(document.getElementById('keyDailyInput').value, 10) || 2000;
  
  const btn = document.getElementById('keyCreateBtn'); btn.disabled = true;
  try {
    const res = await req('/admin/apikeys', { method:'POST', body: JSON.stringify({ name, rate_limit: rate, daily_limit: daily }) });
    document.getElementById('keyNameInput').value = '';
    openModal({
      title: 'API Key Generated', 
      msg: 'Please store this key securely. You will not be able to view it again.', 
      fields: [{ key: 'newKey', label: 'Secret Key', value: res.key, readonly: true }],
      confirmTxt: 'Close',
      onConfirm: () => {}
    });
    refreshKeys();
  } catch(err) { flash(err.message, 'error'); }
  finally { btn.disabled = false; }
});

document.getElementById('configForm').addEventListener('submit', async e => {
  e.preventDefault();
  const body = {
    log_level: document.getElementById('cfg_loglevel').value,
    proxy: {
      cloudflare_timeout: parseInt(document.getElementById('cfg_cf_timeout').value, 10),
      max_auth_retries: parseInt(document.getElementById('cfg_auth_retry').value, 10),
      telegram_bot_token: document.getElementById('cfg_tg_token').value,
      telegram_chat_id: document.getElementById('cfg_tg_chat').value
    }
  };
  const ak = document.getElementById('cfg_app_key').value;
  if(ak) body.app_key = ak;

  const btn = document.getElementById('configSaveBtn'); btn.disabled = true;
  try {
    await req('/admin/config', { method:'PUT', body: JSON.stringify(body) });
    document.getElementById('cfg_app_key').value = '';
    toast('Config saved & overridden');
    refreshConfig();
  } catch(err) { flash(err.message, 'error'); }
  finally { btn.disabled = false; }
});

/* Init */
async function initSession() {
  try {
    await req('/admin/verify');
    document.getElementById('loginView').classList.add('hidden');
    document.getElementById('appView').classList.remove('hidden');
    const hash = window.location.hash.substring(1);
    switchView(hash || 'overview');
  } catch {
    document.getElementById('loginView').classList.remove('hidden');
    document.getElementById('appView').classList.add('hidden');
  }
}

document.addEventListener('DOMContentLoaded', initSession);
