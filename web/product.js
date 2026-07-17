const state = { case: null, orders: [], selectedOrderId: null, drafts: [], liveDraft: null, activeView: 'patient', filter: 'all', identity: null, staticMode: false };
const isStaticHost = location.hostname.endsWith("github.io") || location.protocol === "file:";

const $ = (selector) => document.querySelector(selector);
const escapeHTML = (value = '') => value.replace(/[&<>'"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char]));

async function api(path, options = {}) {
  if (isStaticHost) throw new Error('GitHub Pages 仅托管静态页面，在线演示暂未连接后端 API');
  const response = await fetch(path, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, ...options });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.error || '服务暂时不可用，请稍后重试');
  return payload;
}

async function bootstrapIdentity() {
  if (isStaticHost) {
    state.staticMode = true;
    state.identity = { role: 'patient' };
    return;
  }
  try {
    state.identity = await api('/api/auth/me');
  } catch (_) {
    state.identity = await api('/api/auth/guest', { method: 'POST' });
  }
}

function showPortal(portal) {
  if (state.staticMode && portal === 'doctor') {
    state.identity = { role: 'doctor' };
    state.orders = [];
  }
  state.activeView = portal;
  $('#patientView').classList.toggle('active-view', portal === 'patient');
  $('#doctorView').classList.toggle('active-view', portal === 'doctor');
  document.querySelectorAll('.mode-tab').forEach((tab) => tab.classList.toggle('active', tab.dataset.view === portal));
  if (portal === 'doctor') {
    const loggedIn = state.identity?.role === 'doctor';
    $('#doctorLogin').classList.toggle('hidden', loggedIn);
    $('#doctorDetail').classList.toggle('hidden', !loggedIn);
    $('.orders-panel').classList.toggle('hidden', !loggedIn);
    if (loggedIn && !state.staticMode) loadOrders();
    if (loggedIn && state.staticMode) renderOrders();
  }
}

function showToast(message) {
  const toast = $('#toast');
  toast.textContent = message;
  toast.classList.add('show');
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => toast.classList.remove('show'), 2400);
}

function statusLabel(status) {
  return ({ collecting: '补充信息中', checking: '医生核验中', verified: '核验已完成', preconsult: '预问诊中', waiting_doctor: '等待医生接诊', live: '真人问诊进行中', completed: '问诊已结束', urgent: '紧急就医提醒' }[status] || status);
}

function messageAvatar(message) {
  if (message.role === 'doctor') return '林';
  if (message.role === 'assistant') return '✣';
  return '你';
}

function renderMessages(c) {
  const box = $('#chatMessages');
  box.innerHTML = c.messages.map((message) => {
    const role = escapeHTML(message.role);
    if (role === 'system') return `<div class="message system"><div class="bubble">${escapeHTML(message.content)}</div></div>`;
    return `<div class="message ${role}"><div class="message-avatar">${messageAvatar(message)}</div><div class="bubble-wrap"><div class="message-name">${escapeHTML(message.name)}</div><div class="bubble">${escapeHTML(message.content)}</div></div></div>`;
  }).join('');
  requestAnimationFrame(() => { box.scrollTop = box.scrollHeight; });
}

function renderPatientAction(c) {
  const card = $('#patientActionCard');
  const content = {
    collecting: c.answers.length >= 3 ? ['信息已整理', '3 个补充信息已收齐，现在可以提交给林医生进行真人核验。', '<button data-patient-action="submit-check">提交真人核验</button>'] : ['继续补充信息', `已记录 ${c.answers.length} / 3 条。医生智能助手会根据你的回答继续追问。`, ''],
    checking: ['核验进行中', '林医生正在查看你的搜索上下文和补充信息。你也可以在对话框继续补充。', ''],
    verified: ['继续问诊', '核验已完成。若要继续讨论饮食、用药或复查报告，可发起图文问诊；本次信息自动带入。', '<button data-patient-action="pay">¥39 发起图文问诊</button>'],
    preconsult: ['已支付，正在预问诊', '医生接诊前，智能助手正在整理必要信息。回答后会自动进入待接诊队列。', ''],
    waiting_doctor: ['等待林医生接诊', '预问诊资料已整理完成。林医生接诊后将开启限时图文会话。', ''],
    live: ['真人问诊已开始', '你正在与林医生图文对话。本次会话时长 30 分钟。', ''],
    completed: ['本次问诊已结束', '医生智能助手已在对话中生成问诊小结与后续建议。', ''],
    urgent: ['请优先线下就医', '检测到可能的紧急风险，此时不建议等待线上服务。', ''],
  }[c.status] || ['咨询室', '', ''];
  card.innerHTML = `<span class="mini-label">下一步</span><h3>${content[0]}</h3><p>${content[1]}</p>${content[2]}`;
}

function renderPatient(c) {
  state.case = c;
  $('#patientStart').classList.add('hidden');
  $('#patientRoom').classList.remove('hidden');
  $('#contextQuery').textContent = c.query;
  $('#contextContent').textContent = c.check_content;
  $('#statusBadge').textContent = statusLabel(c.status);
  renderMessages(c);
  renderPatientAction(c);
}

async function createCase(event) {
  event.preventDefault();
  const button = event.currentTarget.querySelector('button');
  button.disabled = true;
  button.textContent = '正在进入咨询室…';
  try {
    const c = await api('/api/mvp/cases', { method: 'POST', body: JSON.stringify({ query: $('#queryInput').value, check_content: $('#checkInput').value }) });
    renderPatient(c);
    await loadOrders();
  } catch (error) { showToast(error.message); }
  finally { button.disabled = false; button.innerHTML = '请医生核一下 <span>→</span>'; }
}

async function patientMessage(event) {
  event.preventDefault();
  if (!state.case) return;
  const input = $('#patientMessageInput');
  const content = input.value.trim();
  if (!content) return;
  input.value = '';
  try {
    renderPatient(await api(`/api/mvp/cases/${state.case.id}/message`, { method: 'POST', body: JSON.stringify({ content }) }));
    await loadOrders();
  } catch (error) { showToast(error.message); input.value = content; }
}

async function runPatientAction(action) {
  if (!state.case) return;
  try {
    const route = action === 'pay' ? 'pay' : 'submit-check';
    renderPatient(await api(`/api/mvp/cases/${state.case.id}/${route}`, { method: 'POST' }));
    await loadOrders();
    showToast(action === 'pay' ? '支付成功，已创建图文问诊' : '已提交给林医生核验');
  } catch (error) { showToast(error.message); }
}

function orderCategory(c) {
  if (c.status === 'checking') return 'checking';
  return 'consult';
}

function renderOrders() {
  const filtered = state.orders.filter((c) => state.filter === 'all' || orderCategory(c) === state.filter);
  $('#orderCount').textContent = state.orders.length;
  $('#ordersList').innerHTML = filtered.length ? filtered.map((c) => `<button class="order-item ${state.selectedOrderId === c.id ? 'active' : ''}" data-order-id="${c.id}" type="button"><header><span class="order-type">${c.status === 'checking' ? '内容核验' : '图文问诊'}</span><time>${statusLabel(c.status)}</time></header><strong>${escapeHTML(c.query)}</strong><p>${c.status === 'checking' ? `已补充 ${c.answers.length} 项信息` : `预问诊已补充 ${c.pre_answers.length} 项 · ${c.department}`}</p></button>`).join('') : '<p class="empty-state">这一类订单暂时为空。</p>';
}

async function loadOrders() {
  if (state.identity?.role !== 'doctor') return;
  try {
    state.orders = await api('/api/mvp/orders');
    renderOrders();
    if (state.selectedOrderId) {
      const selected = state.orders.find((c) => c.id === state.selectedOrderId);
      if (selected) await openOrder(selected.id, true);
    }
  } catch (error) { console.warn(error); }
}

function contextRows(c) {
  const checks = [
    ['搜索 Query', c.query], ['核验内容', c.check_content],
    ...c.answers.map((answer, i) => [`补充信息 ${i + 1}`, answer]),
    ...c.pre_answers.map((answer, i) => [`预问诊 ${i + 1}`, answer]),
  ];
  return checks.map(([key, value]) => `<div class="context-row"><b>${key}</b><span>${escapeHTML(value)}</span></div>`).join('');
}

async function openOrder(id, quiet = false) {
  try {
    const c = await api(`/api/mvp/cases/${id}`);
    state.selectedOrderId = id;
    state.liveDraft = null;
    if (c.status === 'checking') state.drafts = await api(`/api/mvp/cases/${id}/doctor/drafts`);
    else state.drafts = [];
    renderOrders();
    renderDoctorDetail(c);
  } catch (error) { if (!quiet) showToast(error.message); }
}

function renderDoctorDetail(c) {
  const detail = $('#doctorDetail');
  const base = `<header class="detail-head"><div><span class="mini-label">患者服务单</span><h2>${escapeHTML(c.query)}</h2><p>${escapeHTML(c.department)} · ${escapeHTML(c.doctor_name)}</p></div><span class="state-chip">${statusLabel(c.status)}</span></header><div class="detail-content"><section class="patient-context"><h3>患者有效信息</h3>${contextRows(c)}</section>`;
  if (c.status === 'checking') {
    const drafts = state.drafts.map((draft, index) => `<article class="draft ${index === 0 ? 'selected' : ''}" data-draft-index="${index}"><label>${escapeHTML(draft.label)}</label><p>${escapeHTML(draft.content)}</p><button type="button" data-use-draft="${index}">选用此回复</button></article>`).join('');
    detail.innerHTML = `${base}<section class="draft-area"><h3>Copilot 核验建议 <small>左右滑动选择</small></h3><div class="drafts">${drafts}</div><div class="reply-editor"><textarea id="checkReply" rows="5">${escapeHTML(state.drafts[0]?.content || '')}</textarea><div class="action-row"><button class="send-edit" data-doctor-action="send-check-edit" type="button">编辑后发送</button><button class="send-direct" data-doctor-action="send-check-direct" type="button">直接发送</button></div></div></section></div>`;
    return;
  }
  if (c.status === 'preconsult' || c.status === 'waiting_doctor') {
    const copy = c.status === 'preconsult' ? '患者正在回答智能助手的预问诊问题。你可以现在接诊，提前进入真人对话。' : '预问诊已完成，患者正在等待接诊。';
    detail.innerHTML = `${base}<section class="accept-box"><span class="mini-label">图文问诊</span><h3>准备接诊</h3><p>${copy}</p><button data-doctor-action="accept" type="button">接诊并开启 30 分钟会话</button></section></div>`;
    return;
  }
  if (c.status === 'live') {
    detail.innerHTML = `${base}<section class="live-doctor-area"><h3>真人 IM · 已同步患者与智能助手的全部记录</h3><div class="copilot-line"><button id="generateLiveCopilot" type="button">✣ 生成 Copilot 推荐回复</button><span>医生确认后才会发出</span></div>${state.liveDraft ? `<div class="copilot-preview">${escapeHTML(state.liveDraft.content)}</div>` : ''}<div class="doctor-composer"><textarea id="liveReply" placeholder="输入发送给患者的回复…">${escapeHTML(state.liveDraft?.content || '')}</textarea><div class="action-row"><button class="end-button" data-doctor-action="end" type="button">结束问诊</button><button class="send-direct" data-doctor-action="live-send" type="button">发送给患者</button></div></div></section></div>`;
    return;
  }
  detail.innerHTML = `${base}<section class="accept-box"><h3>服务状态已更新</h3><p>${c.status === 'completed' ? '患者端已生成问诊小结。' : '此订单当前无需医生处理。'}</p></section></div>`;
}

async function doctorAction(action) {
  if (!state.selectedOrderId) return;
  const id = state.selectedOrderId;
  try {
    if (action === 'send-check-direct' || action === 'send-check-edit') {
      const content = $('#checkReply').value.trim();
      const c = await api(`/api/mvp/cases/${id}/doctor/send-check`, { method: 'POST', body: JSON.stringify({ content, action: action === 'send-check-edit' ? 'edit' : 'direct' }) });
      if (state.case?.id === id) renderPatient(c);
      showToast('核验回复已发送给患者');
    } else if (action === 'accept') {
      const c = await api(`/api/mvp/cases/${id}/accept`, { method: 'POST' });
      if (state.case?.id === id) renderPatient(c);
      showToast('已接诊，真人会话已开启');
    } else if (action === 'live-send') {
      const content = $('#liveReply').value.trim();
      const c = await api(`/api/mvp/cases/${id}/doctor/message`, { method: 'POST', body: JSON.stringify({ content, action: state.liveDraft ? 'direct' : 'edit' }) });
      state.liveDraft = null;
      if (state.case?.id === id) renderPatient(c);
      showToast('已发送给患者');
    } else if (action === 'end') {
      const c = await api(`/api/mvp/cases/${id}/end`, { method: 'POST' });
      if (state.case?.id === id) renderPatient(c);
      showToast('问诊已结束，已生成问诊小结');
    }
    await loadOrders();
    if (action !== 'end') await openOrder(id, true);
  } catch (error) { showToast(error.message); }
}

async function generateLiveCopilot() {
  if (!state.selectedOrderId) return;
  try {
    state.liveDraft = await api(`/api/mvp/cases/${state.selectedOrderId}/copilot`, { method: 'POST' });
    const c = await api(`/api/mvp/cases/${state.selectedOrderId}`);
    renderDoctorDetail(c);
  } catch (error) { showToast(error.message); }
}

async function doctorLogin(event) {
  event.preventDefault();
  try {
    state.identity = await api('/api/auth/doctor/login', { method: 'POST', body: JSON.stringify({ code: $('#doctorCodeInput').value }) });
    $('#doctorCodeInput').value = '';
    showPortal('doctor');
    showToast('已进入医生工作台');
  } catch (error) { showToast(error.message); }
}

document.addEventListener('click', async (event) => {
  const action = event.target.closest('[data-patient-action]')?.dataset.patientAction;
  if (action) runPatientAction(action);
  const order = event.target.closest('[data-order-id]')?.dataset.orderId;
  if (order) openOrder(order);
  const filter = event.target.closest('.order-filter');
  if (filter) { state.filter = filter.dataset.filter; document.querySelectorAll('.order-filter').forEach((button) => button.classList.toggle('active', button === filter)); renderOrders(); }
  const draft = event.target.closest('[data-use-draft]')?.dataset.useDraft;
  if (draft !== undefined) { const item = state.drafts[Number(draft)]; $('#checkReply').value = item.content; document.querySelectorAll('.draft').forEach((node, index) => node.classList.toggle('selected', index === Number(draft))); }
  const doctorActionName = event.target.closest('[data-doctor-action]')?.dataset.doctorAction;
  if (doctorActionName) doctorAction(doctorActionName);
  if (event.target.closest('#generateLiveCopilot')) generateLiveCopilot();
});

$('#startCaseForm').addEventListener('submit', createCase);
$('#patientMessageForm').addEventListener('submit', patientMessage);
$('#doctorLoginForm').addEventListener('submit', doctorLogin);
$('#refreshOrders').addEventListener('click', loadOrders);
document.querySelectorAll('.mode-tab').forEach((tab) => tab.addEventListener('click', (event) => {
  if (tab.dataset.view === 'doctor') {
    event.preventDefault();
    history.replaceState(null, '', '?view=doctor');
    showPortal('doctor');
  }
}));

bootstrapIdentity()
  .then(() => showPortal(new URLSearchParams(window.location.search).get('view') === 'doctor' || window.location.pathname === '/doctor' ? 'doctor' : 'patient'))
  .catch((error) => showToast(error.message));

setInterval(() => {
  if (state.staticMode) return;
  if (state.case && state.identity?.role === 'patient') api(`/api/mvp/cases/`).then(renderPatient).catch(() => {});
  if (state.activeView === 'doctor') loadOrders();
}, 5000);
