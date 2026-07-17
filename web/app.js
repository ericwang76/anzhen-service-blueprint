const routeCopy = {
  verify: {
    title: '内容核验：原文直达真人医生',
    body: '用户不再经历 AI 补充信息阶段。要验证的内容直接进入真人医生待办，医生基于原始上下文给出确认和建议。',
  },
  partner: {
    title: '合作医生标注反馈：真人问诊 + AI 训练闭环',
    body: '用户支付诊费后直接等待真人接入。合作医生一边问诊，一边对 AI 候选回复做标注和修订，积累可训练反馈。',
  },
  delegated: {
    title: 'AI 托管预问诊：非合作医生也能接入',
    body: '在真人接入前由 AI 自动完成预问诊和摘要整理，真人接入后再切回统一的多模型 Copilot 与语音编辑流程。',
  },
};

const routeCards = Array.from(document.querySelectorAll('[data-route]'));
const routeDetail = document.querySelector('#routeDetail');
const candidateCards = Array.from(document.querySelectorAll('.candidate-card'));
const voiceButtons = Array.from(document.querySelectorAll('.voice-button'));
const toast = document.querySelector('#toast');
const voiceNotes = {
  verify: document.querySelector('#voiceNoteVerify'),
  partner: document.querySelector('#voiceNotePartner'),
  delegated: document.querySelector('#voiceNoteDelegated'),
};

const candidateDetail = {
  'wenxin-a': '偏保守，适合给出稳妥的复查建议。',
  'wenxin-b': '偏口语，适合转成更容易理解的解释。',
  gpt: '结构完整，适合作为最终医生回复底稿。',
  'partner-wenxin-a': '更适合做初版医学表达。',
  'partner-wenxin-b': '更适合做安抚语气的补充。',
  'partner-gpt': '适合输出可直接发送的最终版本。',
  'delegated-wenxin-a': '优先追问关键风险点。',
  'delegated-wenxin-b': '更适合做摘要复述。',
  'delegated-gpt': '适合作为接入后的完整回复草稿。',
};

const voicePrompt = {
  verify: '医生可以直接说：删掉 AI 追问，把语气改得更直接，并加上不要自行用药。',
  partner: '医生可以直接说：保留复查建议，弱化安抚语气，补充家族史风险和就诊阈值。',
  delegated: '医生可以直接说：把回复改成先追问体重变化，再解释 HbA1c 检查意义。',
};

function showToast(message) {
  toast.textContent = message;
  toast.classList.add('show');
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => toast.classList.remove('show'), 2400);
}

function setActiveRoute(route) {
  routeCards.forEach((card) => card.classList.toggle('active', card.dataset.route === route));
  const data = routeCopy[route];
  routeDetail.querySelector('h2').textContent = data.title;
  routeDetail.querySelector('p').textContent = data.body;
}

function setActiveCandidate(card) {
  const model = card.dataset.model;
  candidateCards.forEach((item) => item.classList.toggle('active', item === card));
  showToast(`已切换候选：${card.querySelector('header span').textContent}`);
  const note = candidateDetail[model];
  if (note) {
    const panel = card.closest('.copilot-board');
    const helper = panel?.querySelector('.voice-note, p[id^="voiceNote"]');
    if (helper) helper.textContent = note;
  }
}

function runVoiceEdit(target) {
  const note = voiceNotes[target];
  if (note) note.textContent = `语音已记录：${voicePrompt[target]}`;
  voiceButtons.forEach((button) => button.classList.toggle('recording', button.dataset.voiceTarget === target));
  showToast('已进入语音编辑模式');
}

routeCards.forEach((card) => {
  card.addEventListener('click', () => setActiveRoute(card.dataset.route));
});

candidateCards.forEach((card) => {
  card.addEventListener('click', () => setActiveCandidate(card));
});

voiceButtons.forEach((button) => {
  button.addEventListener('click', () => runVoiceEdit(button.dataset.voiceTarget));
});

setActiveRoute('verify');
setActiveCandidate(candidateCards.find((card) => card.classList.contains('active')) || candidateCards[0]);
showToast('蓝图已切换到三条服务链路版本');
