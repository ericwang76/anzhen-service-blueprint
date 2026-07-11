const steps = ["step-p1", "step-p2", "bridge-submit", "step-d1", "step-d2", "step-d3", "bridge-response", "step-p4"];
const viewport = document.querySelector("#blueprintViewport");
const playButton = document.querySelector("#playFlowBtn");
const toast = document.querySelector("#toast");
const editButton = document.querySelector("#editReplyBtn");
const replyText = document.querySelector("#doctorReplyText");
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

editButton.addEventListener("click", () => {
  replyText.readOnly = !replyText.readOnly;
  editButton.textContent = replyText.readOnly ? "编辑" : "完成";
  if (!replyText.readOnly) replyText.focus();
});

function focusStep(id, announce = true) {
  document.querySelectorAll(".phone-step").forEach((step) => step.classList.remove("active-step"));
  document.querySelectorAll(".handoff-line").forEach((bridge) => bridge.classList.remove("active-bridge"));
  const target = document.getElementById(id);
  if (!target) return;

  if (target.classList.contains("phone-step")) target.classList.add("active-step");
  if (target.classList.contains("handoff-line")) target.classList.add("active-bridge");

  const canvasRect = document.querySelector(".blueprint-canvas").getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const nextLeft = viewport.scrollLeft + targetRect.left - canvasRect.left - (viewport.clientWidth - targetRect.width) / 2;
  viewport.scrollTo({ left: Math.max(0, nextLeft), behavior: "smooth" });

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
  }[id] || "当前流程步骤";
}

function showToast(message) {
  window.clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add("show");
  toastTimer = window.setTimeout(() => toast.classList.remove("show"), 2200);
}
