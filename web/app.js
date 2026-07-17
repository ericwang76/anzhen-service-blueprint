const steps = ["step-p1", "step-p2", "bridge-submit", "step-d1", "step-d2", "step-d3", "bridge-response", "step-p4", "step-p5", "consult-payment-handoff", "step-p6", "step-d4", "step-d5", "consult-join-handoff", "step-p7", "step-p8", "step-d6", "step-p9"];
const viewport = document.querySelector("#blueprintViewport");
const playButton = document.querySelector("#playFlowBtn");
const toast = document.querySelector("#toast");
const editButton = document.querySelector("#editReplyBtn");
const replyText = document.querySelector("#doctorReplyText");
const draftButtons = document.querySelectorAll(".full-draft[data-reply]");
const voiceEditButtons = document.querySelectorAll(".voice-edit");
const openDoctorImButton = document.querySelector("#openDoctorImBtn");
const closeDoctorImButton = document.querySelector("#closeDoctorImBtn");
const doctorImSheet = document.querySelector("#doctorImSheet");
const doctorImBackdrop = document.querySelector("#doctorImBackdrop");
const doctorLiveInput = document.querySelector("#doctorLiveInput");
const copilotReply = "水果可以吃，建议放在两餐之间，每次约一个拳头大小；优先苹果、梨或莓果，先避免果汁和高糖水果。";
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
  showToast("已打开与林医生的真人对话，预问诊记录已同步");
});

function closeDoctorIm() {
  doctorImSheet?.classList.remove("open");
  doctorImBackdrop?.classList.remove("open");
}

closeDoctorImButton?.addEventListener("click", closeDoctorIm);
doctorImBackdrop?.addEventListener("click", closeDoctorIm);

document.querySelector("#acceptConsultBtn")?.addEventListener("click", () => {
  focusStep("consult-join-handoff");
  window.setTimeout(() => focusStep("step-p7", false), 900);
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
draftButtons.forEach((button) => {
  button.addEventListener("click", () => {
    draftButtons.forEach((draft) => {
      draft.classList.remove("selected");
      const badge = draft.querySelector("header b");
      if (badge) badge.textContent = "可选";
    });
    button.classList.add("selected");
    const badge = button.querySelector("header b");
    if (badge) badge.textContent = "✓ 已选择";
    if (replyText && button.dataset.reply) replyText.value = button.dataset.reply;
    showToast("已切换为当前模型候选回复，可继续语音编辑");
  });
});

function applyVoiceEdit() {
  if (!replyText) return;
  replyText.value = "这段百科内容基本准确。你这次空腹血糖 6.3 mmol/L 比常用参考上限略高，但一次检查不能诊断糖尿病，需要复查确认。\n\n建议先保持正常饮食和作息，1～2 周内复查空腹血糖，同时加查糖化血红蛋白（HbA1c）。如果复查仍偏高，或出现口渴、多尿、体重下降，再到内分泌科进一步评估。";
  replyText.classList.add("voice-edited");
  setTimeout(() => replyText.classList.remove("voice-edited"), 900);
  showToast("已按语音要求弱化诊断语气，并补充复查条件");
}

editButton?.addEventListener("click", applyVoiceEdit);
voiceEditButtons.forEach((button) => button.addEventListener("click", (event) => {
  event.stopPropagation();
  applyVoiceEdit();
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
    "step-p2": "进入患者咨询室：搜索上下文已自动带入",
    "bridge-submit": "跨端提交：服务单已发送给真人医生",
    "step-d1": "医生收到完整核验服务单",
    "step-d2": "Copilot 已生成两种回答方案",
    "step-d3": "医生审核、编辑并承担最终发送动作",
    "bridge-response": "跨端回传：医生回答送达患者咨询室",
    "step-p4": "患者获得核验结论，并可继续真人问诊",
    "step-p5": "支付成功：问诊订单已创建，等待医生接诊",
    "step-p6": "医生智能助手正在进行接诊前预问诊",
    "consult-payment-handoff": "付费问诊订单与完整上下文已送达医生端",
    "step-d4": "医生从多类型订单中心定位付费问诊",
    "step-d5": "医生查看付费问诊订单与预问诊摘要",
    "consult-join-handoff": "医生已接诊，患者端即将开启真人 IM",
    "step-p7": "医生接入卡已推送到患者咨询室",
    "step-p8": "患者与医生正在限时真人 IM 中多轮沟通",
    "step-d6": "医生端真人会话进行中，Copilot 可推荐回复",
    "step-p9": "问诊结束，智能助手已生成小结与指导建议",
  }[id] || "当前流程步骤";
}

function showToast(message) {
  window.clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add("show");
  toastTimer = window.setTimeout(() => toast.classList.remove("show"), 2200);
}
