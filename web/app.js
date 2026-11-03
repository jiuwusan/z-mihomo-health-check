'use strict';

const $ = id => document.getElementById(id);
let loaded = false;
let selectedInstance = '';
let instanceList = [];
let instanceEpoch = 0;
let managementBusy = false;
let dirty = false;
let eventSignature = '';
let resultSignature = '';
let groupResults = [];
let currentGroup = '';
let currentRow = null;
let historyCursor = 0;
let nextHistoryCursor = 0;
let historyID = '';
let historyRequest = 0;
let historyTotal = 0;
let displayedEvents = [];
let panelRequest = 0;
let instanceRequests = new AbortController();
let pollTimer = null;
let pollTask = null;
let pollDelay = 2000;
const phaseNames = {stopped:'探查已停用',waiting:'等待探查',checking:'正在探查',healthy:'出口正常',warning:'探查失败',testing:'整组测试中',switching:'正在切换',error:'需要关注'};
async function api(path, options = {}) {
  const response = await fetch(path, {...options, headers:{'Content-Type':'application/json',...options.headers}, signal:options.signal || AbortSignal.timeout(6000)});
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || `请求失败（${response.status}）`);
  return body;
}
function instanceAPI(path, options = {}, id = selectedInstance) {
  if (!id) return Promise.reject(new Error('请先选择实例'));
  const {timeoutMs = 6000, signal:callerSignal, ...requestOptions} = options;
  const signals = [instanceRequests.signal, AbortSignal.timeout(timeoutMs)];
  if (callerSignal) signals.push(callerSignal);
  return api('/api/instances/' + encodeURIComponent(id) + path.replace(/^\/api/, ''), {...requestOptions, signal:AbortSignal.any(signals)});
}
function syncInstanceControls() {
  $('instance-select').disabled = managementBusy || !instanceList.length;
  $('add-instance').disabled = managementBusy || !loaded;
  $('rename-instance').disabled = managementBusy || !loaded;
  $('delete-instance').disabled = managementBusy || !loaded || instanceList.length < 2;
  $('instance-fields').disabled = managementBusy || !loaded;
  $('open-settings').disabled = managementBusy || !loaded;
}
async function refreshInstances() {
  const data = await api('/api/instances');
  instanceList = data.instances;
  $('instance-select').replaceChildren(...instanceList.map(item => new Option(item.name, item.id)));
  $('instance-select').value = selectedInstance;
  $('instance-count').textContent = instanceList.length + ' 个实例';
  syncInstanceControls();
}
function allowDiscard() {return !dirty || confirm('当前实例有未保存的配置，是否放弃修改？');}
async function switchInstance(id, force = false) {
  if (id === selectedInstance && loaded && !force) return true;
  if (!force && !allowDiscard()) {$('instance-select').value = selectedInstance; return false;}
  const epoch = ++instanceEpoch;
  pausePolling();
  pollDelay = 2000;
  instanceRequests.abort();
  instanceRequests = new AbortController();
  selectedInstance = id; loaded = false; dirty = false;
  message('panel-message','');
  ++panelRequest; ++historyRequest;
  historyCursor = 0; nextHistoryCursor = 0; historyID = ''; historyTotal = 0; displayedEvents = []; eventSignature = '';
  $('instance-select').value = id;
  $('instance-name').value = instanceList.find(item => item.id === id)?.name || id;
  $('group-search').value = '';
  $('open-settings').textContent = '探查配置 ↗';
  $('secret').value = ''; $('clear-secret').checked = false;
  resetWebhookFields();
  $('target-group').replaceChildren();
  $('connect-button').disabled = false; $('connect-button').textContent = '测试连接并加载分组 ↗';
  $('save-button').textContent = '保存配置 →'; $('save-button').disabled = true;
  message('save-message',''); message('connection-message','连接后可选择 API 返回的 select 分组。'); message('history-error','');
  renderStatus({phase:'waiting',message:'正在加载实例配置',pool_size:200,failures:0,results:[]});
  renderHistoryEvents(); $('older-events').disabled = true; $('latest-events').disabled = true;
  syncInstanceControls();
  try {
    const cfg = await instanceAPI('/api/config', {}, id);
    if (epoch !== instanceEpoch) return false;
    applyConfig(cfg); await poll(); return true;
  } catch (error) {
    if (epoch === instanceEpoch) {message('save-message',error.message,true); message('panel-message',error.message,true); $('monitor-badge').textContent = '实例加载失败';}
    return false;
  }
}
$('instance-select').addEventListener('change', () => {switchInstance($('instance-select').value);});
async function manageInstance(action) {
  if (managementBusy || !loaded) return;
  const id = selectedInstance;
  if (action !== 'rename' && !allowDiscard()) return;
  if (action === 'delete' && !confirm('删除该实例并停止探查？已保存的数据将保留备份。')) return;
  managementBusy = true; syncInstanceControls(); message('instance-message','');
  try {
    let created;
    const name = $('instance-name').value.trim();
    if (action === 'add') created = await api('/api/instances',{method:'POST',body:JSON.stringify({name})});
    if (action === 'rename') await api('/api/instances/' + encodeURIComponent(id),{method:'PUT',body:JSON.stringify({name})});
    if (action === 'delete') await api('/api/instances/' + encodeURIComponent(id),{method:'DELETE'});
    await refreshInstances();
    if (action === 'rename') $('instance-name').value = instanceList.find(item => item.id === id).name;
    else await switchInstance(created ? created.id : instanceList[0].id, true);
    message('instance-message',action === 'add' ? '实例已添加，填写 API 配置后保存即可。' : action === 'rename' ? '实例已重命名。' : '实例已删除，后台探查已停止。');
  } catch (error) {message('instance-message',error.message,true);}
  finally {managementBusy = false; syncInstanceControls();}
}
$('add-instance').addEventListener('click', () => {manageInstance('add');});
$('rename-instance').addEventListener('click', () => {manageInstance('rename');});
$('delete-instance').addEventListener('click', () => {manageInstance('delete');});
function message(id, text, error = false) {
  const el = $(id); el.textContent = text;
  el.classList.toggle('error', error); el.classList.toggle('success', !error && Boolean(text));
}
function payload() {
  return {api_url:$('api-url').value.trim(), secret:$('secret').value, clear_secret:$('clear-secret').checked,
    dingtalk_webhook:$('dingtalk-webhook').value.trim(), feishu_webhook:$('feishu-webhook').value.trim(),
    clear_dingtalk_webhook:$('clear-dingtalk-webhook').checked, clear_feishu_webhook:$('clear-feishu-webhook').checked,
    group:$('target-group').value, interval_seconds:Number($('interval').value),
    failure_threshold:Number($('threshold').value), timeout_seconds:Number($('timeout').value), enabled:$('enabled').checked,
    persist_history:$('persist-history').checked, recovery_enabled:$('recovery-enabled').checked,
    recovery_initial_delay_seconds:Number($('recovery-initial-delay').value),
    recovery_interval_seconds:Number($('recovery-interval').value), recovery_success_threshold:Number($('recovery-threshold').value)};
}
function ensureGroup(name) {
  const select = $('target-group');
  if (![...select.options].some(o => o.value === name)) select.add(new Option(name, name));
  select.value = name;
}
function applyConfig(cfg) {
  resetWebhookFields(cfg);
  $('api-url').value = cfg.api_url; ensureGroup(cfg.group);
  $('interval').value = cfg.interval_seconds; $('threshold').value = cfg.failure_threshold; $('timeout').value = cfg.timeout_seconds;
  $('enabled').checked = cfg.enabled;
  $('persist-history').checked = Boolean(cfg.persist_history);
  $('recovery-enabled').checked = Boolean(cfg.recovery_enabled);
  $('recovery-initial-delay').value = cfg.recovery_initial_delay_seconds;
  $('recovery-interval').value = cfg.recovery_interval_seconds;
  $('recovery-threshold').value = cfg.recovery_success_threshold;
  $('secret').value = ''; $('secret').disabled = false; $('clear-secret').checked = false;
  $('secret-state').textContent = cfg.secret_set ? '已保存 · 不回显' : '未设置';
  $('failure-limit').textContent = ` / ${cfg.failure_threshold} 次`;
  $('config-summary').textContent = `${cfg.group} · 每 ${cfg.interval_seconds} 秒探查`;
  $('config-summary').title = $('config-summary').textContent;
  $('open-settings').textContent = '探查配置 ↗';
  loaded = true; dirty = false; $('save-button').disabled = false; syncInstanceControls();
}
function resetWebhookFields(cfg = {}) {
  for (const channel of ['dingtalk','feishu']) {
    $(channel + '-webhook').value = '';
    $(channel + '-webhook').disabled = false;
    $('clear-' + channel + '-webhook').checked = false;
    $(channel + '-webhook-state').textContent = cfg[channel + '_webhook_set'] ? '已保存 · 不回显' : '未设置';
  }
}
for (const channel of ['dingtalk','feishu']) {
  $('clear-' + channel + '-webhook').addEventListener('change', () => {
    $(channel + '-webhook').disabled = $('clear-' + channel + '-webhook').checked;
  });
}
function element(tag, className, text) {
  const node = document.createElement(tag); if (className) node.className = className; if (text !== undefined) node.textContent = text; return node;
}
function clock(value) {return value ? new Date(value).toLocaleTimeString('zh-CN',{hour12:false}) : '—';}
function renderStatus(status) {
  const badge = $('monitor-badge'); badge.textContent = phaseNames[status.phase] || status.phase;
  badge.className = 'badge' + (status.running && status.phase !== 'error' ? ' active' : status.phase === 'error' ? ' error' : '');
  $('current-group').textContent = status.current || '—'; $('delay').textContent = status.delay_ms > 0 ? status.delay_ms : '—';
  $('current-group').title = status.current || '';
  $('failures').textContent = status.failures; $('pool-size').textContent = status.pool_size;
  $('phase-text').title = status.message || '';
  $('phase-text').textContent = status.message; $('last-check').textContent = status.last_check ? `最近探查 ${clock(status.last_check)}` : '尚未探查';
  const lastSite = $('last-site');
  lastSite.textContent = status.last_site || '尚未开始探查';
  lastSite.title = status.last_site || '';
  if (/^https?:\/\//i.test(status.last_site || '')) lastSite.href = status.last_site;
  else lastSite.removeAttribute('href');
  $('activity-summary').textContent = status.last_switch ? `最近 1 小时记录 · 最近切换 ${clock(status.last_switch)}` : '仅保留最近 1 小时记录';
  const recovery = status.recovery || {};
  const recoveryProgress = recovery.initial_wait_seconds > 0
    ? `首次检查等待 ${recovery.initial_wait_seconds} 秒`
    : recovery.candidate ? `候选 ${recovery.candidate} · 连续成功 ${recovery.successes}/${recovery.threshold} 次` : recovery.message || '等待恢复检查';
  $('recovery-status').textContent = recovery.enabled
    ? recoveryProgress
    : '自动恢复未运行';
  $('recovery-status').title = $('recovery-status').textContent;
  const results = status.results || [];
  const resultKey = JSON.stringify([results,status.current]);
  if (resultKey !== resultSignature) {
    resultSignature = resultKey;
    groupResults = results;
    currentGroup = status.current || '';
    renderGroups();
  }
}
function renderGroups() {
  const query = $('group-search').value.trim().toLocaleLowerCase();
  const visible = groupResults.map((result,index) => ({result,index})).filter(({result}) => result.name.toLocaleLowerCase().includes(query));
  $('group-summary').textContent = groupResults.length ? `${groupResults.filter(result => result.available).length} 可用 / ${groupResults.length} 个分组` : '等待探查';
  $('locate-current').disabled = !groupResults.some(result => result.name === currentGroup);
  const scrollTop = $('priority-list').scrollTop;
  currentRow = null;
  $('priority-list').replaceChildren(...(visible.length ? visible.map(({result,index}) => {
    const tested = Boolean(result.checked_at);
    const row = element('div',`priority-row${result.available ? ' available' : ''}${tested ? '' : ' untested'}${result.name === currentGroup ? ' selected' : ''}`);
    row.append(element('span','rank',String(index + 1).padStart(2,'0')),element('span','country',result.name));
    if (result.name === currentGroup) {row.append(element('span','selected-label','当前')); currentRow = row;}
    const availability = element('span','availability',result.available ? `${result.delay_ms} ms` : tested ? '最近探查失败' : '尚未探查');
    if (tested) {
      const timestamp = element('time','result-time',`检测于 ${clock(result.checked_at)}`);
      timestamp.dateTime = result.checked_at;
      timestamp.title = new Date(result.checked_at).toLocaleString('zh-CN', {hour12:false});
      availability.append(timestamp);
    }
    row.append(availability); return row;
  }) : [element('div','empty-events',query ? '没有匹配的分组，请尝试其他名称。' : '等待分组探查，结果将在这里显示。')]));
  $('priority-list').scrollTop = scrollTop;
}
$('group-search').addEventListener('input', () => {renderGroups(); $('priority-list').scrollTop = 0;});
$('locate-current').addEventListener('click', () => {
  $('group-search').value = ''; renderGroups();
  if (currentRow) currentRow.scrollIntoView({block:'nearest'});
});
for (const name of ['settings','instances']) {
  $('open-' + name).addEventListener('click', () => {
    if (!$(name + '-dialog').open) $(name + '-dialog').showModal();
  });
  $('close-' + name).addEventListener('click', () => {$(name + '-dialog').close();});
}

function renderHistoryEvents() {
  const cutoff = Date.now() - 60 * 60 * 1000;
  displayedEvents = displayedEvents.filter(event => new Date(event.time).getTime() >= cutoff);
  const events = displayedEvents; const signature = JSON.stringify(events);
  if (signature !== eventSignature) {
    eventSignature = signature;
    $('event-list').replaceChildren(...(events.length ? events.map(event => {
      const row = element('div',`event-row ${event.level}`);
      row.append(element('time','event-time',clock(event.time)),element('span','event-dot'),element('span','event-message',event.message));return row;
    }) : [element('div','empty-events',historyCursor ? '当前页记录已过期，请返回最新。' : '最近 1 小时暂无记录。')]));
  }
  $('history-summary').textContent = `最近 1 小时 · ${historyTotal} 条 · 本页 ${events.length} 条`;
}
async function refreshHistory(status, signal) {
  if (historyID !== status.history_id) {historyID = status.history_id; historyCursor = 0;}
  const request = ++historyRequest;
  const epoch = instanceEpoch; const id = selectedInstance;
  const cursor = historyCursor;
  const page = cursor ? await instanceAPI(`/api/events?before=${cursor}`, {signal}, id) : {
    events:status.events || [], total:status.event_count || 0, history_id:status.history_id,
    has_more:(status.event_count || 0) > (status.events || []).length,
    next_before:(status.events || []).at(-1)?.id || 0
  };
  if (signal?.aborted || epoch !== instanceEpoch || request !== historyRequest || cursor !== historyCursor) return;
  if (page.history_id !== historyID) {historyCursor = 0; return;}
  displayedEvents = page.events; historyTotal = page.total; nextHistoryCursor = page.next_before;
  $('older-events').disabled = !page.has_more;
  $('latest-events').disabled = cursor === 0;
  message('history-error','');
  renderHistoryEvents();
}
async function refreshPanel(signal) {
  const request = ++panelRequest;
  const epoch = instanceEpoch; const id = selectedInstance;
  ++historyRequest;
  try {
    const status = await instanceAPI('/api/status', {signal}, id);
    if (signal?.aborted || epoch !== instanceEpoch || request !== panelRequest) return;
    renderStatus(status); await refreshHistory(status, signal);
  } catch (error) {
    if (epoch === instanceEpoch && request === panelRequest) throw error;
  }
}
$('older-events').addEventListener('click', async () => {
  if (!nextHistoryCursor) return;
  historyCursor = nextHistoryCursor;
  $('older-events').disabled = true;
  try {await refreshPanel(); $('event-list').scrollTop = 0;}
  catch (error) {message('history-error',error.message,true); $('latest-events').disabled = false;}
});
$('latest-events').addEventListener('click', async () => {
  historyCursor = 0; ++historyRequest;
  try {await refreshPanel(); $('event-list').scrollTop = 0;}
  catch (error) {message('history-error',error.message,true);}
});
$('settings-form').addEventListener('input', () => {
  if (loaded) {dirty = true; $('open-settings').textContent = '配置 · 未保存'; message('save-message','有未保存的修改');}
});
$('clear-secret').addEventListener('change', () => {
  $('secret').disabled = $('clear-secret').checked;
});
$('connect-button').addEventListener('click', async () => {
  if (!$('settings-form').reportValidity()) return;
  const epoch = instanceEpoch; const id = selectedInstance;
  const button = $('connect-button'); button.disabled = true; button.textContent = '正在连接…';
  message('connection-message','正在测试连接，不会保存配置或启用探查。');
  try {
    const draft = payload();
    const data = await instanceAPI('/api/groups',{method:'POST',body:JSON.stringify(draft),timeoutMs:(draft.timeout_seconds+4)*1000},id);
    if (epoch !== instanceEpoch) return;
    const select = $('target-group'); select.replaceChildren(...data.groups.map(g => new Option(g.name,g.name)));
    if (data.groups.some(g => g.name === draft.group)) select.value = draft.group;
    message('connection-message',data.groups.length ? `连接成功，已加载 ${data.groups.length} 个 select 分组。` : '连接成功，但没有可管理的 select 分组。',!data.groups.length);
    if (!data.groups.length) ensureGroup(draft.group);
  } catch (error) {if (epoch === instanceEpoch) message('connection-message',error.message,true);}
  finally {if (epoch === instanceEpoch) {button.disabled = false; button.textContent = '测试连接并加载分组 ↗';}}
});
$('settings-form').addEventListener('submit', async event => {
  event.preventDefault(); if (!loaded) return;
  const epoch = instanceEpoch; const id = selectedInstance;
  const button = $('save-button'); button.disabled = true; button.textContent = '正在保存…';
  try {
    const cfg = await instanceAPI('/api/config',{method:'PUT',body:JSON.stringify(payload())},id);
    if (epoch !== instanceEpoch) return;
    applyConfig(cfg);
    message('save-message',cfg.enabled ? '配置已保存，后台自动探查已启用。' : '配置已保存，后台自动探查已停用。');
    await refreshPanel();
  } catch (error) {if (epoch === instanceEpoch) message('save-message',error.message,true);}
  finally {if (epoch === instanceEpoch) {button.disabled = false; button.textContent = '保存配置 →';}}
});
function pausePolling() {
  clearTimeout(pollTimer);
  pollTimer = null;
  if (pollTask) {
    const task = pollTask;
    pollTask = null;
    task.controller.abort();
  }
}
async function poll() {
  if (!loaded || document.hidden || pollTask) return;
  clearTimeout(pollTimer);
  pollTimer = null;
  const task = {controller:new AbortController(), epoch:instanceEpoch};
  pollTask = task;
  try {
    await refreshPanel(task.controller.signal);
    if (pollTask !== task || task.epoch !== instanceEpoch) return;
    pollDelay = 2000;
  } catch {
    if (pollTask !== task || task.controller.signal.aborted || task.epoch !== instanceEpoch) return;
    pollDelay = Math.min(pollDelay * 2, 30000);
    const badge = $('monitor-badge');badge.textContent = '面板连接中断';badge.className = 'badge error';
  } finally {
    // A canceled or previous-instance request cannot restart the polling loop.
    if (pollTask === task) {
      pollTask = null;
      renderHistoryEvents();
      if (loaded && !document.hidden) pollTimer = setTimeout(poll,pollDelay);
    }
  }
}
document.addEventListener('visibilitychange', () => {
  if (document.hidden) pausePolling();
  else void poll();
});
async function initialize() {
  try {await refreshInstances(); await switchInstance(instanceList[0].id,true);}
  catch (error) {message('panel-message','无法加载配置，请刷新页面重试。',true);$('monitor-badge').textContent = '面板连接中断';$('monitor-badge').className = 'badge error';}
}
initialize();
