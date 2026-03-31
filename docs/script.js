const COPY_ICON = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`;
const CHECK_ICON = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>`;

function makeCopyButton(getText) {
  const btn = document.createElement('button');
  btn.className = 'code-copy-btn';
  btn.setAttribute('aria-label', 'Copy');
  btn.innerHTML = COPY_ICON;

  btn.addEventListener('click', () => {
    navigator.clipboard.writeText(getText()).then(() => {
      btn.innerHTML = CHECK_ICON;
      btn.classList.add('copied');
      setTimeout(() => {
        btn.innerHTML = COPY_ICON;
        btn.classList.remove('copied');
      }, 2000);
    });
  });

  return btn;
}

function wrapWithCopyButton(el, getText) {
  const wrap = document.createElement('div');
  wrap.className = 'code-copy-wrap';
  el.parentNode.insertBefore(wrap, el);
  wrap.appendChild(el);
  wrap.appendChild(makeCopyButton(getText));
}

function attachCopyButtons() {
  // Tab code blocks
  document.querySelectorAll('.code-block').forEach(pre => {
    if (pre.parentElement.classList.contains('code-copy-wrap')) return;
    wrapWithCopyButton(pre, () => pre.innerText.replace(/\n$/, ''));
  });

  // Inline code lines in install section
  document.querySelectorAll('.code-line').forEach(line => {
    if (line.parentElement.classList.contains('code-copy-wrap')) return;
    const code = line.querySelector('code');
    if (!code) return;
    wrapWithCopyButton(line, () => code.textContent.trim());
  });
}

// Hero install copy button (wired via onclick in HTML)
function copyInstall() {
  const text = document.getElementById('install-cmd').textContent;
  const copyIcon = document.getElementById('copy-icon');
  const checkIcon = document.getElementById('check-icon');

  navigator.clipboard.writeText(text).then(() => {
    copyIcon.style.display = 'none';
    checkIcon.style.display = 'block';
    setTimeout(() => {
      copyIcon.style.display = 'block';
      checkIcon.style.display = 'none';
    }, 2000);
  });
}

function showTab(name) {
  document.querySelectorAll('.tab').forEach(t => {
    t.classList.remove('active');
    t.setAttribute('aria-selected', 'false');
  });
  document.querySelectorAll('.code-block').forEach(b => b.classList.remove('active'));

  const tab = [...document.querySelectorAll('.tab')].find(t => t.getAttribute('onclick') === `showTab('${name}')`);
  const block = document.getElementById('tab-' + name);
  if (tab) { tab.classList.add('active'); tab.setAttribute('aria-selected', 'true'); }
  if (block) block.classList.add('active');
}

document.addEventListener('DOMContentLoaded', attachCopyButtons);
