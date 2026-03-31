function copyInstall() {
  const text = document.getElementById('install-cmd').textContent;
  navigator.clipboard.writeText(text).then(() => {
    const copyIcon = document.getElementById('copy-icon');
    const checkIcon = document.getElementById('check-icon');
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
