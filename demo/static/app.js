'use strict';

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const icon = (name) => `<svg aria-hidden="true"><use href="#${name}"/></svg>`;
const escapeHTML = (value) => String(value ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const field = (name, label, placeholder, options = {}) => ({ name, label, placeholder, ...options });
const key = () => field('key', 'Key', '例如 user:1', { required: true });
const value = () => field('value', 'Value', '输入要存储的内容，可留空');
const member = () => field('member', 'Member', '例如 Tom', { required: true });
const hashField = () => field('field', 'Field', '例如 name', { required: true });
const integer = (name, label, placeholder, extra = {}) => field(name, label, placeholder, { type: 'number', required: true, ...extra });

const commands = [
  { name: 'SET', group: 'String', description: '设置字符串，可选择仅在键不存在时写入，或设置过期时间。', fields: [key(), value()], options: true },
  { name: 'GET', group: 'String', description: '读取指定 key 对应的字符串。', fields: [key()] },
  { name: 'MSET', group: 'String', description: '一次设置多组 key 和 value。', repeat: [key(), value()], addLabel: '添加一组键值' },
  { name: 'MGET', group: 'String', description: '一次读取多个 key，按输入顺序返回结果。', repeat: [key()], addLabel: '添加一个 key' },
  { name: 'LPUSH', group: 'List', description: '将一个或多个元素插入列表左侧。', fields: [key()], repeat: [value()], addLabel: '添加一个元素' },
  { name: 'RPUSH', group: 'List', description: '将一个或多个元素插入列表右侧。', fields: [key()], repeat: [value()], addLabel: '添加一个元素' },
  { name: 'LPOP', group: 'List', description: '从列表左侧弹出元素，不填写数量时默认弹出一个。', fields: [key(), integer('count', 'Count', '可选，默认为 1', { required: false, min: 1 })] },
  { name: 'RPOP', group: 'List', description: '从列表右侧弹出元素，不填写数量时默认弹出一个。', fields: [key(), integer('count', 'Count', '可选，默认为 1', { required: false, min: 1 })] },
  { name: 'LRANGE', group: 'List', description: '读取下标闭区间内的列表元素，0 到 -1 表示整个列表。', fields: [key(), integer('start', 'Start', '起始下标', { initial: '0', min: 0 }), integer('stop', 'Stop', '结束下标，-1 表示末尾', { initial: '-1', min: -1 })] },
  { name: 'SADD', group: 'Set', description: '添加集合成员。重复的成员只保存一份。', fields: [key()], repeat: [member()], addLabel: '添加一个成员' },
  { name: 'SISMEMBER', group: 'Set', description: '判断集合是否包含指定成员，1 表示存在，0 表示不存在。', fields: [key(), member()] },
  { name: 'SREM', group: 'Set', description: '删除一个或多个集合成员。', fields: [key()], repeat: [member()], addLabel: '添加一个成员' },
  { name: 'HSET', group: 'Hash', description: '设置 Hash 中的一个或多个字段。', fields: [key()], repeat: [hashField(), value()], addLabel: '添加一组字段' },
  { name: 'HGET', group: 'Hash', description: '读取 Hash 中指定字段的值。', fields: [key(), hashField()] },
  { name: 'HDEL', group: 'Hash', description: '删除 Hash 中的一个或多个字段。', fields: [key()], repeat: [hashField()], addLabel: '添加一个字段' },
  { name: 'ZADD', group: 'Sorted Set', description: '添加成员及整数分数；成员已存在时更新其分数。', fields: [key()], repeat: [integer('score', 'Score', '整数分数'), member()], addLabel: '添加一组成员' },
  { name: 'ZRANGEBYSCORE', group: 'Sorted Set', description: '按分数升序读取闭区间内的成员，上界 -1 表示无上限。', fields: [key(), integer('min', 'Min', '最低分数', { initial: '0' }), integer('max', 'Max', '最高分数', { initial: '-1' })] },
  { name: 'ZREM', group: 'Sorted Set', description: '删除有序集合中的一个或多个成员。', fields: [key()], repeat: [member()], addLabel: '添加一个成员' },
  { name: 'EXPIRE', group: '过期', description: '为已有的 key 设置过期时间，单位为秒。', fields: [key(), integer('seconds', 'Seconds', '例如 60', { min: 1 })] },
  { name: 'EXPIREAT', group: '过期', description: '设置本地时区的绝对过期时间。本项目使用日期字符串。', fields: [key(), field('time', 'Date & time', '例如 2026-12-31 23:59:59', { required: true, pattern: '\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}', hint: '格式：YYYY-MM-DD HH:MM:SS' })] },
];
const types = [
  { name: 'string', label: 'String', color: '#8bb7ed' },
  { name: 'list', label: 'List', color: '#b3a0e3' },
  { name: 'set', label: 'Set', color: '#87c4ab' },
  { name: 'hash', label: 'Hash', color: '#e3bb86' },
  { name: 'zset', label: 'Sorted Set', color: '#d8a0b8' },
];
const state = { entries: [], history: [], selected: commands[0], filtered: commands, group: '', logTab: 'history', storageKey: null, lastResult: null };
let fieldCounter = 0;
let toastTimer;
let wheelFrame;

async function api(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { 'Content-Type': 'application/json', ...options.headers } });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || '请求失败，请稍后重试');
  return data;
}
function toast(message) {
  $('#toast').textContent = message;
  $('#toast').classList.add('visible');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => $('#toast').classList.remove('visible'), 3500);
}
function showModal(id) {
  $$('dialog[open]').forEach((dialog) => dialog.close());
  $(id).showModal();
}
function readable(value) {
  if (value === null || value === undefined) return '(nil)';
  if (typeof value === 'object') return JSON.stringify(value, null, 2);
  if (value === '') return '""（空字符串）';
  return String(value);
}
function commandText(args) {
  return args.map((arg) => !arg || /\s|["\\]/.test(arg) ? JSON.stringify(arg) : arg).join(' ');
}
function localTime(value, full = false) {
  return new Date(value).toLocaleString('zh-CN', full ? { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false } : { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}
function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
function badge(type) {
  const label = types.find((item) => item.name === type)?.label || type;
  return `<span class="type-badge badge-${escapeHTML(type)}">${escapeHTML(label)}</span>`;
}

async function refreshHealth() {
  try {
    const health = await api('/api/health');
    $('#connection').classList.toggle('offline', !health.connected);
    $('#connection-text').textContent = health.connected ? '本地服务已连接' : '数据库未连接';
    $('#footer-address').textContent = health.address;
    $('#aof-label').textContent = health.appendOnly ? 'AOF 已开启' : 'AOF 未开启';
    return health.connected;
  } catch (error) {
    $('#connection').classList.add('offline');
    $('#connection-text').textContent = '服务未连接';
    return false;
  }
}
async function refreshStorage() {
  const response = await api('/api/storage');
  state.entries = response.entries;
  $('#storage-key-count').textContent = state.entries.length;
  $('#key-total').textContent = state.entries.length;
  $('#expiry-count').textContent = `${state.entries.filter((entry) => entry.expiresAt).length} 个键设置了过期时间`;
  const counts = types.map((type) => state.entries.filter((entry) => entry.type === type.name).length);
  let cursor = 0;
  const segments = types.map((type, index) => {
    const start = cursor;
    cursor += counts[index] / Math.max(state.entries.length, 1) * 100;
    return `${type.color} ${start}% ${cursor}%`;
  });
  $('#type-donut').style.background = state.entries.length ? `conic-gradient(${segments.join(',')})` : '#f0f0f4';
  $('#type-legend').innerHTML = types.map((type, index) => `<div class="legend-row"><i style="background:${type.color}"></i><span>${type.label}</span><b>${counts[index]}</b></div>`).join('');
  $('#storage-summary').textContent = `${state.entries.length} 个有效键 · 更新于 ${localTime(response.time)}`;
  renderStorage();
  if (state.storageKey) renderDetail(state.storageKey);
}
async function refreshHistory() {
  state.history = await api('/api/history');
  const recent = state.history.slice(-5).reverse();
  if (recent.length) {
    $('#recent-list').innerHTML = recent.map((item) => `<div class="recent-item"><span class="record-status ${item.ok ? '' : 'failed'}">${icon(item.ok ? 'check' : 'close')}</span><div><div class="record-title">${escapeHTML(commandText(item.args))}</div><div class="record-result">${escapeHTML(readable(item.value))}</div></div><div class="record-time">${localTime(item.time)}<span>${item.durationMs.toFixed(2)} ms</span></div></div>`).join('');
  } else {
    $('#recent-list').innerHTML = `<div class="empty-state"><span class="empty-icon">${icon('terminal')}</span><h3>从一条 SET 开始</h3><p>执行第一条命令，结果将在这里出现。</p><button class="text-button" id="first-set">试试 SET ${icon('arrow')}</button></div>`;
  }
  if ($('#logs-dialog').open && state.logTab === 'history') renderHistory();
}

async function refreshAOF() {
  const data = await api('/api/aof');
  const syncLabel = { always: '每次写入', everysec: '每秒同步', no: '由系统决定' }[data.syncPolicy] || data.syncPolicy || '未配置';
  $('#aof-card-status').textContent = data.enabled ? '已开启' : '未开启';
  $('#aof-card-size').textContent = data.enabled && data.exists ? formatBytes(data.sizeBytes) : '暂无文件';
  $('#aof-enabled').textContent = data.enabled ? '已开启' : '未开启';
  $('#aof-sync').textContent = syncLabel;
  $('#aof-size').textContent = data.exists ? formatBytes(data.sizeBytes) : '暂无文件';
  $('#aof-total').textContent = data.exists ? `${data.total} 条` : '0 条';
  $('#aof-file-info').textContent = `${data.fileName === '.' ? '未配置文件' : data.fileName} · ${data.updatedAt ? `更新于 ${localTime(data.updatedAt)}` : '尚未生成'}`;
  $('#aof-records').innerHTML = data.commands.length ? data.commands.slice().reverse().map((item) => `<div class="aof-record"><span>#${item.number}</span><code>${escapeHTML(commandText(item.args))}</code></div>`).join('') : '<div class="table-empty">当前 AOF 文件还没有命令。先执行一条 SET，再刷新这里。</div>';
  const note = data.readError ? `读取提示：${data.readError}` : data.partialTail ? '文件尾部正在写入，刷新后可查看完整记录。' : !data.enabled ? '在 redis.conf 中设置 appendonly yes 后重启 Demo，即可启用 AOF。' : `当前使用 ${syncLabel} 策略。AOF 重写阈值：${data.rewriteAfter || '未启用'} 条写命令。重写后文件记录会合并。`;
  $('#aof-note').textContent = note;
  return data;
}
async function openAOF() {
  showModal('#aof-dialog');
  try { await refreshAOF(); } catch (error) { $('#aof-records').innerHTML = `<div class="table-empty">${escapeHTML(error.message)}</div>`; }
}

function openPicker() {
  $('#command-search').value = '';
  state.group = '';
  renderCategories();
  renderWheel();
  showModal('#picker-dialog');
  requestAnimationFrame(() => {
    positionWheel(state.filtered.indexOf(state.selected));
    $('#command-wheel').focus({ preventScroll: true });
  });
}
function renderCategories() {
  $('#categories').innerHTML = ['', 'String', 'List', 'Set', 'Hash', 'Sorted Set', '过期'].map((group) => `<button data-group="${group}" class="${state.group === group ? 'active' : ''}">${group || '全部'}</button>`).join('');
}
function renderWheel() {
  const search = $('#command-search').value.trim().toLowerCase();
  state.filtered = commands.filter((command) => (!state.group || command.group === state.group) && (command.name.toLowerCase().includes(search) || command.description.includes(search)));
  if (!state.filtered.includes(state.selected)) state.selected = state.filtered[0] || commands[0];
  $('#command-wheel').innerHTML = state.filtered.map((command) => `<button class="wheel-row" role="option" aria-selected="${state.selected === command}" data-command="${command.name}" tabindex="-1"><span>${command.name}</span><small>${command.group}</small></button>`).join('');
  $('#choose-command').disabled = !state.filtered.length;
  $('#selected-category').textContent = state.filtered.length ? state.selected.group.toUpperCase() : 'NO RESULTS';
  $('#selected-description').textContent = state.filtered.length ? state.selected.description : '没有找到匹配的命令，换个关键词试试。';
  positionWheel(Math.max(state.filtered.indexOf(state.selected), 0));
}
function positionWheel(index, smooth = false) {
  $('#command-wheel').scrollTo({ top: Math.max(0, index) * 42, behavior: smooth ? 'smooth' : 'instant' });
  if (state.filtered[index]) selectWheel(index);
}
function selectWheel(index) {
  if (!state.filtered[index]) return;
  state.selected = state.filtered[index];
  $$('.wheel-row').forEach((row) => row.setAttribute('aria-selected', String(row.dataset.command === state.selected.name)));
  $('#selected-category').textContent = state.selected.group.toUpperCase();
  $('#selected-description').textContent = state.selected.description;
}
$('#command-wheel').addEventListener('scroll', () => {
  cancelAnimationFrame(wheelFrame);
  wheelFrame = requestAnimationFrame(() => selectWheel(Math.min(state.filtered.length - 1, Math.max(0, Math.round($('#command-wheel').scrollTop / 42)))));
});
$('#command-wheel').addEventListener('click', (event) => {
  const row = event.target.closest('[data-command]');
  if (row) positionWheel(state.filtered.findIndex((command) => command.name === row.dataset.command), true);
});
$('#picker-dialog').addEventListener('keydown', (event) => {
  if (event.key === 'Enter' && state.filtered.length) { event.preventDefault(); openCommand(state.selected); }
  if (event.target.id === 'command-search') return;
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const index = Math.max(0, Math.min(state.filtered.length - 1, state.filtered.indexOf(state.selected) + (event.key === 'ArrowDown' ? 1 : -1)));
    positionWheel(index);
  }
});
$('#command-search').addEventListener('input', renderWheel);
$('#categories').addEventListener('click', (event) => {
  const button = event.target.closest('[data-group]');
  if (!button) return;
  state.group = button.dataset.group;
  renderCategories(); renderWheel();
});

function inputHTML(spec) {
  const id = `argument-${++fieldCounter}`;
  const attributes = [spec.required ? 'required' : '', spec.min !== undefined ? `min="${spec.min}"` : '', spec.pattern ? `pattern="${spec.pattern}"` : ''].filter(Boolean).join(' ');
  return `<div class="form-field"><label for="${id}">${spec.label}${!spec.required && spec.name === 'count' ? '<span>可选</span>' : ''}</label><input id="${id}" data-arg="${spec.name}" type="${spec.type || 'text'}" placeholder="${escapeHTML(spec.placeholder)}" value="${escapeHTML(spec.initial || '')}" ${spec.type === 'number' ? 'step="1"' : ''} ${attributes} autocomplete="off">${spec.hint ? `<p class="form-hint">${spec.hint}</p>` : ''}</div>`;
}
function addRepeatRow(focus = true) {
  const row = document.createElement('div');
  row.className = 'pair-row';
  row.innerHTML = state.selected.repeat.map(inputHTML).join('') + `<button type="button" class="icon-button remove-row" aria-label="删除这一组参数">${icon('close')}</button>`;
  $('#repeat-rows').append(row);
  updateRemoveButtons();
  updatePreview();
  if (focus) $('input', row).focus();
}
function updateRemoveButtons() {
  const rows = $$('.pair-row', $('#repeat-rows') || document.createElement('div'));
  rows.forEach((row) => $('.remove-row', row).disabled = rows.length <= 1);
}
function openCommand(command, preset = {}) {
  state.selected = command;
  $('#command-title').textContent = command.name;
  $('#command-type').textContent = command.group.toUpperCase();
  $('#command-description').textContent = command.description;
  $('#form-error').textContent = '';
  const fields = $('#command-fields');
  fields.innerHTML = (command.fields || []).map(inputHTML).join('');
  if (command.repeat) {
    fields.insertAdjacentHTML('beforeend', `<div id="repeat-rows" style="display:flex;flex-direction:column;gap:12px"></div><button type="button" class="text-button add-row" id="add-row">${icon('plus')}${command.addLabel}</button>`);
    addRepeatRow(false);
  }
  if (command.options) {
    fields.insertAdjacentHTML('beforeend', `<div class="option-block"><div class="option-row"><label for="option-nx"><span class="option-name">NX</span><span class="option-note">仅在 key 不存在时写入</span></label><input type="checkbox" role="switch" class="switch" id="option-nx"></div><div class="option-row"><label for="option-ex"><span class="option-name">EX</span><span class="option-note">设置过期时间</span></label><input type="checkbox" role="switch" class="switch" id="option-ex"></div><div class="ttl-input form-field" id="ttl-row" hidden><input type="number" min="1" step="1" placeholder="例如 60" id="ttl-seconds" aria-label="过期秒数" disabled><span>秒</span></div></div>`);
  }
  Object.entries(preset).forEach(([name, val]) => { const input = $(`[data-arg="${name}"]`, fields); if (input) input.value = val; });
  updatePreview();
  showModal('#command-dialog');
  requestAnimationFrame(() => $('input[data-arg]', fields)?.focus());
}
function collectArgs(preview = false) {
  const args = [state.selected.name];
  $$('input[data-arg]', $('#command-fields')).forEach((input) => {
    if (input.dataset.arg === 'count' && !input.value) return;
    args.push(input.value || (preview && input.required ? `<${input.dataset.arg}>` : ''));
  });
  if ($('#option-nx')?.checked) args.push('NX');
  if ($('#option-ex')?.checked) args.push('EX', $('#ttl-seconds').value || (preview ? '<seconds>' : ''));
  return args;
}
function updatePreview() { $('#command-preview').textContent = commandText(collectArgs(true)); }
$('#command-fields').addEventListener('input', updatePreview);
$('#command-fields').addEventListener('change', (event) => {
  if (event.target.id === 'option-ex') {
    $('#ttl-row').hidden = !event.target.checked;
    $('#ttl-seconds').disabled = !event.target.checked;
    $('#ttl-seconds').required = event.target.checked;
    if (event.target.checked) $('#ttl-seconds').focus();
  }
  updatePreview();
});
$('#command-fields').addEventListener('click', (event) => {
  if (event.target.closest('#add-row')) addRepeatRow();
  const remove = event.target.closest('.remove-row');
  if (remove && !remove.disabled) { remove.closest('.pair-row').remove(); updateRemoveButtons(); updatePreview(); }
});
$('#command-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!event.target.reportValidity()) return;
  const button = $('#execute-command');
  if (button.disabled) return;
  const args = collectArgs();
  button.disabled = true;
  button.textContent = '执行中…';
  $('#form-error').textContent = '';
  try {
    const result = await api('/api/command', { method: 'POST', body: JSON.stringify({ args }) });
    state.lastResult = result;
    renderResult(result);
    showModal('#result-dialog');
    const refreshed = await Promise.allSettled([refreshStorage(), refreshHistory(), refreshHealth(), refreshAOF()]);
    if (refreshed.some((item) => item.status === 'rejected')) toast('命令已执行，部分面板刷新失败，可手动重试');
  } catch (error) {
    $('#form-error').textContent = error.message;
  } finally {
    button.disabled = false;
    button.innerHTML = '执行命令 <kbd>↵</kbd>';
  }
});
function renderResult(result) {
  $('#result-title').textContent = result.ok ? '执行完成。' : '这次没有成功。';
  $('#result-body').innerHTML = `<div class="result-command">${escapeHTML(commandText(result.args))}</div><div class="result-meta"><span class="type-badge ${result.ok ? '' : 'failed'}">${result.ok ? result.type.toUpperCase() : 'ERROR'}</span><span>${result.durationMs.toFixed(2)} ms</span><span>${localTime(result.time)}</span></div><pre class="result-value">${escapeHTML(readable(result.value))}</pre><details class="result-raw"><summary>查看原始 RESP 响应</summary><pre>${escapeHTML(result.raw.replace(/\r/g, '\\r').replace(/\n/g, '\\n\n') || '未收到 RESP 响应')}</pre></details>`;
}

async function openStorage() {
  state.storageKey = null;
  $('#storage-detail').hidden = true;
  showModal('#storage-dialog');
  try { await refreshStorage(); } catch (error) { toast(error.message); }
}
function renderStorage() {
  const search = $('#storage-search').value.toLowerCase();
  const type = $('#storage-type').value;
  const entries = state.entries.filter((entry) => (!type || entry.type === type) && (!search || entry.key.toLowerCase().includes(search) || readable(entry.value).toLowerCase().includes(search)));
  $('#storage-rows').innerHTML = entries.map((entry) => `<tr data-key="${escapeHTML(entry.key)}" tabindex="0" aria-label="查看 ${escapeHTML(entry.key)}" class="${state.storageKey === entry.key ? 'selected' : ''}"><td title="${escapeHTML(entry.key)}">${escapeHTML(entry.key)}</td><td>${badge(entry.type)}</td><td>${escapeHTML(readable(entry.value))}</td><td>${entry.expiresAt ? escapeHTML(localTime(entry.expiresAt, true)) : '永不过期'}</td><td>${icon('chevron')}</td></tr>`).join('');
  $('#storage-empty').hidden = entries.length > 0;
  $('#storage-empty').textContent = state.entries.length ? '没有匹配的内容。' : '数据库还很安静，执行一条 SET 写入第一份数据吧。';
}
function renderDetail(keyName) {
  const entry = state.entries.find((item) => item.key === keyName);
  if (!entry) { state.storageKey = null; $('#storage-detail').hidden = true; return; }
  state.storageKey = keyName;
  $('#storage-detail').hidden = false;
  $('#storage-detail').innerHTML = `<div class="detail-heading"><h3>${escapeHTML(entry.key)}</h3><button class="icon-button" id="close-detail" aria-label="关闭数据详情">${icon('close')}</button></div><div class="detail-meta">${badge(entry.type)} &nbsp; ${entry.size} ${entry.type === 'string' ? '字节' : entry.type === 'hash' ? '个字段' : '个元素'} &nbsp; · &nbsp; ${entry.expiresAt ? `过期于 ${escapeHTML(new Date(entry.expiresAt).toLocaleString('zh-CN'))}` : '永不过期'}</div><pre>${escapeHTML(readable(entry.value))}</pre>`;
  renderStorage();
}
$('#storage-rows').addEventListener('click', (event) => { const row = event.target.closest('[data-key]'); if (row) renderDetail(row.dataset.key); });
$('#storage-rows').addEventListener('keydown', (event) => { if (event.key === 'Enter') { const row = event.target.closest('[data-key]'); if (row) renderDetail(row.dataset.key); } });
$('#storage-detail').addEventListener('click', (event) => { if (event.target.closest('#close-detail')) { state.storageKey = null; $('#storage-detail').hidden = true; renderStorage(); } });
$('#storage-search').addEventListener('input', renderStorage);
$('#storage-type').addEventListener('change', renderStorage);

async function openLogs(tab = 'history') {
  state.logTab = tab;
  showModal('#logs-dialog');
  await loadLogs();
}
function renderHistory() {
  const items = state.history.slice().reverse();
  $('#logs-content').innerHTML = items.length ? items.map((item) => `<article class="history-record"><div class="history-record-top"><span class="record-status ${item.ok ? '' : 'failed'}">${icon(item.ok ? 'check' : 'close')}</span><code>${escapeHTML(commandText(item.args))}</code><time>${localTime(item.time)}</time></div><div class="history-value">${escapeHTML(readable(item.value))}<span class="muted"> &nbsp; · &nbsp; ${item.durationMs.toFixed(2)} ms</span></div></article>`).join('') : '<div class="table-empty">还没有命令记录，下一次探索将从这里开始。</div>';
  $('#logs-summary').textContent = `${items.length} 条命令记录 · 当前演示会话`;
}
async function loadLogs() {
  $('#history-tab').setAttribute('aria-selected', String(state.logTab === 'history'));
  $('#runtime-tab').setAttribute('aria-selected', String(state.logTab === 'runtime'));
  $('#log-level').hidden = state.logTab !== 'runtime';
  $('#clear-history').hidden = state.logTab !== 'history';
  $('#logs-content').innerHTML = '<div class="table-empty">正在读取…</div>';
  try {
    if (state.logTab === 'history') { await refreshHistory(); renderHistory(); }
    else {
      const response = await api(`/api/logs?level=${encodeURIComponent($('#log-level').value)}`);
      $('#logs-content').innerHTML = response.lines.length ? response.lines.slice().reverse().map((line) => `<div class="runtime-line ${line.includes('\tERROR\t') ? 'error' : line.includes('\tWARN\t') ? 'warn' : ''}">${escapeHTML(line)}</div>`).join('') : '<div class="table-empty">当前级别暂无运行日志。</div>';
      $('#logs-summary').textContent = `${response.lines.length} 条运行日志 · app.log`;
    }
  } catch (error) { $('#logs-content').innerHTML = `<div class="table-empty">${escapeHTML(error.message)}</div>`; }
}

$('#open-commands').addEventListener('click', openPicker);
$('#open-storage').addEventListener('click', openStorage);
$('#open-logs').addEventListener('click', () => openLogs());
$('#open-aof').addEventListener('click', openAOF);
$('#refresh-aof').addEventListener('click', () => refreshAOF().then(() => toast('AOF 文件已刷新')).catch((error) => toast(error.message)));
$('#aof-command').addEventListener('click', openPicker);
$('#all-history').addEventListener('click', () => openLogs());
$('#recent-list').addEventListener('click', (event) => { if (event.target.closest('#first-set')) openCommand(commands[0]); });
$('#choose-command').addEventListener('click', () => openCommand(state.selected));
$('#change-command').addEventListener('click', openPicker);
$('#next-command').addEventListener('click', () => openCommand(state.selected));
$('#result-storage').addEventListener('click', openStorage);
$('#refresh-storage').addEventListener('click', () => refreshStorage().then(() => toast('存储信息已刷新')).catch((error) => toast(error.message)));
$('#refresh-overview').addEventListener('click', () => refreshStorage().then(() => toast('数据库概览已刷新')).catch((error) => toast(error.message)));
$('#connection').addEventListener('click', () => refreshHealth().then((connected) => toast(connected ? 'goredis 连接正常' : '无法连接数据库，请检查服务')));
$('#history-tab').addEventListener('click', () => { state.logTab = 'history'; loadLogs(); });
$('#runtime-tab').addEventListener('click', () => { state.logTab = 'runtime'; loadLogs(); });
$('#refresh-logs').addEventListener('click', loadLogs);
$('#log-level').addEventListener('change', loadLogs);
$('#clear-history').addEventListener('click', async () => {
  try { await api('/api/history', { method: 'DELETE' }); await refreshHistory(); toast('命令记录已清空，数据库和运行日志不受影响'); }
  catch (error) { toast(error.message); }
});
$$('.close-dialog').forEach((button) => button.addEventListener('click', () => button.closest('dialog').close()));
$$('dialog').forEach((dialog) => dialog.addEventListener('click', (event) => {
  if (event.target !== dialog) return;
  const box = dialog.getBoundingClientRect();
  if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) dialog.close();
}));

// Pointer movement gently tilts the three glass tiles and positions their light.
if (window.matchMedia('(hover: hover) and (pointer: fine)').matches &&
    !window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
  $$('.tilt-card').forEach((card) => {
    let frame = 0;
    card.addEventListener('pointermove', (event) => {
      const bounds = card.getBoundingClientRect();
      const x = event.clientX - bounds.left;
      const y = event.clientY - bounds.top;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        card.style.setProperty('--rx', `${((0.5 - y / bounds.height) * 5).toFixed(2)}deg`);
        card.style.setProperty('--ry', `${((x / bounds.width - 0.5) * 5).toFixed(2)}deg`);
        card.style.setProperty('--mx', `${x.toFixed(0)}px`);
        card.style.setProperty('--my', `${y.toFixed(0)}px`);
      });
    });
    card.addEventListener('pointerleave', () => {
      cancelAnimationFrame(frame);
      card.style.setProperty('--rx', '0deg');
      card.style.setProperty('--ry', '0deg');
    });
  });
}

Promise.allSettled([refreshHealth(), refreshStorage(), refreshHistory(), refreshAOF()]).then((results) => {
  if (results.some((result) => result.status === 'rejected')) toast('部分信息加载失败，可点击刷新重试');
});
setInterval(refreshHealth, 15000);
