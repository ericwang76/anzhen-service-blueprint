const steps = ["step-p1", "step-p2", "bridge-submit", "step-d1", "step-d2", "bridge-response", "step-p4", "step-p5", "consult-payment-handoff", "step-d4", "step-d5", "consult-join-handoff", "step-p8", "step-d6", "step-p9", "step-p10", "step-p11", "step-d7"];
const viewport = document.querySelector("#blueprintViewport");
const playButton = document.querySelector("#playFlowBtn");
const toast = document.querySelector("#toast");
const voiceEditButtons = document.querySelectorAll(".voice-edit");
const openDoctorImButton = document.querySelector("#openDoctorImBtn");
const closeDoctorImButton = document.querySelector("#closeDoctorImBtn");
const doctorImSheet = document.querySelector("#doctorImSheet");
const doctorImBackdrop = document.querySelector("#doctorImBackdrop");
const doctorLiveInput = document.querySelector("#doctorLiveInput");
let copilotReply = "水果可以吃，建议放在两餐之间，每次约一个拳头大小；优先苹果、梨或莓果，先避免果汁和高糖水果。";
let playTimer;
let toastTimer;
let playing = false;

document.querySelectorAll(".flow-trigger").forEach((button) => {
  button.addEventListener("click", () => {
    focusStep(button.dataset.target);
    if (button.dataset.target === "bridge-response") {
      window.setTimeout(() => focusStep("step-p4", false), 900);
    }
  });
});

document.querySelectorAll(".compact-draft").forEach((draft, index) => {
  draft.addEventListener("click", () => {
    document.querySelectorAll(".compact-draft").forEach((item) => {
      item.classList.remove("selected");
      const stateLabel = item.querySelector("header > b");
      if (stateLabel) stateLabel.textContent = "选择";
    });
    draft.classList.add("selected");
    const selectedLabel = draft.querySelector("header > b");
    if (selectedLabel) selectedLabel.textContent = "✓ 已选择";
    replyText.value = [...draft.querySelectorAll("p")].map((paragraph) => paragraph.textContent).join("\n\n");
    const label = document.querySelector(".quick-send-bar small");
    if (label) label.textContent = `已选回答 ${index === 0 ? "A" : "B"}`;
  });
});

document.querySelectorAll(".chat-composer").forEach((form) => {
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const input = form.querySelector("input");
    const message = input.value.trim();
    if (!message) return;

    const chatBody = form.previousElementSibling;
    let liveDialogue = chatBody.querySelector(".live-dialogue");
    if (!liveDialogue) {
      liveDialogue = document.createElement("div");
      liveDialogue.className = "live-dialogue";
      chatBody.appendChild(liveDialogue);
    }
    liveDialogue.hidden = false;

    const userMessage = document.createElement("div");
    userMessage.className = "live-user-message";
    userMessage.textContent = message;
    const assistantMessage = document.createElement("div");
    assistantMessage.className = "live-ai-message";
    const mark = document.createElement("i");
    mark.textContent = "✣";
    const reply = document.createElement("p");
    reply.textContent = form.dataset.reply;
    assistantMessage.append(mark, reply);
    liveDialogue.append(userMessage, assistantMessage);
    input.value = "";
    window.requestAnimationFrame(() => chatBody.scrollTo({ top: chatBody.scrollHeight, behavior: "smooth" }));
  });
});

document.querySelector(".consult-card .cream-button")?.addEventListener("click", () => focusStep("step-p5"));

openDoctorImButton?.addEventListener("click", () => {
  doctorImSheet.classList.add("open");
  doctorImBackdrop.classList.add("open");
  showToast("已打开与林医生的真人对话，前序核验记录已同步");
});

function closeDoctorIm() {
  doctorImSheet?.classList.remove("open");
  doctorImBackdrop?.classList.remove("open");
}

closeDoctorImButton?.addEventListener("click", closeDoctorIm);
doctorImBackdrop?.addEventListener("click", closeDoctorIm);

document.querySelector("#acceptConsultBtn")?.addEventListener("click", () => {
  focusStep("consult-join-handoff");
  window.setTimeout(() => focusStep("step-p8", false), 900);
});

document.querySelector("#endConsultBtn")?.addEventListener("click", () => {
  showToast("问诊已结束，智能助手正在生成问诊小结");
  window.setTimeout(() => focusStep("step-p9", false), 700);
});

document.querySelector("#copilotInsertBtn")?.addEventListener("click", () => {
  if (!doctorLiveInput) return;
  doctorLiveInput.value = copilotReply;
  doctorLiveInput.focus();
  showToast("Copilot 推荐回复已插入，可继续编辑");
});

document.querySelector("#copilotSendBtn")?.addEventListener("click", () => {
  const messageList = document.querySelector(".doctor-live-im .doctor-im-scroll");
  if (!messageList) return;
  const message = document.createElement("div");
  message.className = "doctor-side-message";
  message.textContent = copilotReply;
  messageList.appendChild(message);
  if (doctorLiveInput) doctorLiveInput.value = "";
  messageList.scrollTo({ top: messageList.scrollHeight, behavior: "smooth" });
  showToast("已以医生身份发送 Copilot 推荐回复");
});

document.querySelectorAll(".live-model-card[data-live-reply]").forEach((card) => {
  card.addEventListener("click", () => {
    const scope = card.closest(".doctor-copilot-reply");
    scope?.querySelectorAll(".live-model-card").forEach((item) => {
      item.classList.remove("selected");
      const label = item.querySelector("small");
      if (label) label.textContent = "可选";
    });
    card.classList.add("selected");
    const label = card.querySelector("small");
    if (label) label.textContent = "✓ 已选";
    copilotReply = card.dataset.liveReply || copilotReply;
    showToast("已选中当前模型候选，可直接发送或语音修订");
  });
});

document.querySelectorAll(".live-model-card[data-check-reply]").forEach((card) => {
  card.addEventListener("click", () => {
    const scope = card.closest(".doctor-copilot-reply");
    scope?.querySelectorAll(".live-model-card").forEach((item) => {
      item.classList.remove("selected");
      const label = item.querySelector("small");
      if (label) label.textContent = "可选";
    });
    card.classList.add("selected");
    const label = card.querySelector("small");
    if (label) label.textContent = "✓ 已选";
    showToast("已选择当前模型候选，可直接发送或语音改写");
  });
});

playButton.addEventListener("click", () => {
  if (playing) {
    stopTour();
    return;
  }
  playing = true;
  playButton.textContent = "■ 停止演示";
  let index = 0;
  focusStep(steps[index], false);
  playTimer = window.setInterval(() => {
    index += 1;
    if (index >= steps.length) {
      stopTour();
      showToast("全流程演示完成");
      return;
    }
    focusStep(steps[index], false);
  }, 1400);
});
voiceEditButtons.forEach((button) => button.addEventListener("click", (event) => {
  event.stopPropagation();
  if (button.dataset.voiceTarget === "live") {
    const card = button.closest(".live-model-card");
    if (card?.dataset.liveReply) copilotReply = card.dataset.liveReply;
    copilotReply = "水果可以适量吃，尽量安排在两餐之间，每次一个拳头大小即可。可以优先选完整水果，先不喝果汁；后续再结合复查结果一起调整。";
    if (doctorLiveInput) doctorLiveInput.value = copilotReply;
    showToast(`正在根据 ${button.dataset.source || "当前候选"} 的语音要求生成新版本`);
    return;
  }
  const candidate = button.closest(".live-model-card");
  const scope = candidate?.closest(".doctor-copilot-reply");
  scope?.querySelectorAll(".live-model-card").forEach((item) => {
    item.classList.remove("selected");
    const label = item.querySelector("small");
    if (label) label.textContent = "可选";
  });
  candidate?.classList.add("selected");
  const label = candidate?.querySelector("small");
  if (label) label.textContent = "✓ 已选 · 语音修订中";
  showToast(`正在根据 ${button.dataset.source || "当前候选"} 的语音要求生成新版本`);
}));

function focusStep(id, announce = true) {
  document.querySelectorAll(".phone-step").forEach((step) => step.classList.remove("active-step"));
  document.querySelectorAll(".handoff-line, .extension-handoff").forEach((bridge) => bridge.classList.remove("active-bridge"));
  const target = document.getElementById(id);
  if (!target) return;

  if (target.classList.contains("phone-step")) target.classList.add("active-step");
  if (target.classList.contains("handoff-line") || target.classList.contains("extension-handoff")) target.classList.add("active-bridge");

  const localViewport = target.closest(".blueprint-viewport, .extension-viewport") || viewport;
  const localCanvas = target.closest(".blueprint-canvas, .extension-canvas") || document.querySelector(".blueprint-canvas");
  const canvasRect = localCanvas.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const nextLeft = localViewport.scrollLeft + targetRect.left - canvasRect.left - (localViewport.clientWidth - targetRect.width) / 2;
  localViewport.scrollTo({ left: Math.max(0, nextLeft), behavior: "smooth" });
  const section = target.closest(".consultation-extension, .blueprint-viewport");
  if (section) window.scrollTo({ top: section.getBoundingClientRect().top + window.scrollY - 78, behavior: "smooth" });

  if (announce) showToast(stepMessage(id));
}

function stopTour() {
  window.clearInterval(playTimer);
  playing = false;
  playButton.textContent = "▶ 演示全流程";
}

function stepMessage(id) {
  return {
    "step-p2": "搜索上下文已送达林医生，正在真人核验",
    "bridge-submit": "跨端提交：服务单已发送给真人医生",
    "step-d1": "医生收到完整核验服务单",
    "step-d2": "Copilot 已生成 2 个文心 lite 与 1 个 gpt-5.6sol 候选",
    "bridge-response": "跨端回传：医生回答送达患者咨询室",
    "step-p4": "患者获得核验结论，并可继续真人问诊",
    "step-p5": "支付成功：同一咨询室将从等待状态切换到医生接入卡",
    "consult-payment-handoff": "付费问诊订单与完整上下文已送达医生端",
    "step-d4": "医生从多类型订单中心定位付费问诊",
    "step-d5": "合作医生查看问诊上下文，并参与 AI 标注反馈",
    "consult-join-handoff": "医生已接诊，患者端即将开启真人 IM",
    "step-p8": "患者与医生正在限时真人 IM 中多轮沟通",
    "step-d6": "真人会话中，医生可从三路模型候选中选择并语音修订",
    "step-p9": "问诊结束，智能助手已生成小结与指导建议",
    "step-p10": "非合作医生未接入前，AI 自动完成托管预问诊",
    "step-p11": "真人医生接入后，AI 托管追问立即停止",
    "step-d7": "非合作医生也可使用同一套多模型 Copilot 与语音修订",
  }[id] || "当前流程步骤";
}

function showToast(message) {
  window.clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add("show");
  toastTimer = window.setTimeout(() => toast.classList.remove("show"), 2200);
}
