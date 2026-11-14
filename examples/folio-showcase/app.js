'use strict';

// This is a browser-only product tour. It never calls the Folio API.
const scopeNames = { private: 'Private', organization: 'Organization', selected: 'Selected members', public: 'Public' };
const viewerNames = { visitor: 'A signed-out visitor', owner: 'The artifact owner', admin: 'An administrator', selected: 'A selected active member', member: 'Another active member' };
const scopeRules = {
  private: 'Only the owner and administrators have access.',
  organization: 'Active organization members can open it when signed in.',
  selected: 'Only selected active members, the owner, and administrators have access.',
  public: 'Anyone who can reach your Folio server can open it. Internet access still requires configured ingress.'
};
const result = document.querySelector('#access-result');
const viewer = document.querySelector('#viewer');
function updateAccess() {
  const scope = document.querySelector('input[name="visibility"]:checked').value;
  const role = viewer.value;
  const allowed = role === 'owner' || role === 'admin' || scope === 'public' || (scope === 'organization' && role !== 'visitor') || (scope === 'selected' && role === 'selected');
  document.querySelector('#scope-badge').textContent = scopeNames[scope];
  document.querySelector('#result-title').textContent = allowed ? 'This viewer can open it.' : (scope === 'private' ? 'This artifact is private.' : 'This viewer cannot open it.');
  document.querySelector('#result-description').textContent = `${viewerNames[role]} ${allowed ? 'has' : 'does not have'} access. ${scopeRules[scope]}`;
  result.classList.toggle('allowed', allowed);
  result.querySelector('use').setAttribute('href', allowed ? '#i-check' : '#i-lock');
  result.classList.remove('updated');
  void result.offsetWidth;
  result.classList.add('updated');
}
document.querySelectorAll('input[name="visibility"]').forEach(input => input.addEventListener('change', updateAccess));
viewer.addEventListener('change', updateAccess);

let toastTimeout;
function notify(message) {
  const toast = document.querySelector('#toast');
  clearTimeout(toastTimeout);
  toast.textContent = message;
  toast.hidden = false;
  toastTimeout = setTimeout(() => { toast.hidden = true; }, 4500);
}
async function copyText(value, successMessage) {
  try {
    await navigator.clipboard.writeText(value);
    notify(successMessage);
  } catch {
    // Sandboxed artifacts may not have clipboard permission. Keep the text
    // available for manual selection without navigating or losing tour state.
    let fallback = document.querySelector('#copy-fallback');
    if (!fallback) {
      fallback = document.createElement('section');
      fallback.id = 'copy-fallback';
      fallback.className = 'copy-fallback';
      const heading = document.createElement('h2');
      heading.textContent = 'Copy this text';
      const explanation = document.createElement('p');
      explanation.textContent = 'Clipboard access is unavailable in this viewer. Select the text below and copy it manually.';
      const field = document.createElement('textarea');
      field.setAttribute('aria-label', 'Text to copy');
      field.readOnly = true;
      field.rows = 8;
      const close = document.createElement('button');
      close.className = 'button secondary';
      close.type = 'button';
      close.textContent = 'Close';
      close.addEventListener('click', () => { fallback.hidden = true; });
      fallback.append(heading, explanation, field, close);
      document.querySelector('main').append(fallback);
    }
    fallback.hidden = false;
    const field = fallback.querySelector('textarea');
    field.value = value;
    fallback.scrollIntoView({ block: 'center', behavior: 'smooth' });
    field.focus({ preventScroll: true });
    field.select();
    notify('Select the text below to copy it manually.');
  }
}
const tabs = [...document.querySelectorAll('[role="tab"]')];
function selectTab(tab) {
  tabs.forEach(item => {
    const active = item === tab;
    item.setAttribute('aria-selected', String(active));
    item.tabIndex = active ? 0 : -1;
    document.querySelector(`#${item.getAttribute('aria-controls')}`).hidden = !active;
  });
  document.querySelector('.copy-code').setAttribute('aria-label', `Copy ${tab.textContent} example`);
}
tabs.forEach((tab, index) => {
  tab.addEventListener('click', () => selectTab(tab));
  tab.addEventListener('keydown', event => {
    let next;
    if (event.key === 'ArrowRight') next = tabs[(index + 1) % tabs.length];
    if (event.key === 'ArrowLeft') next = tabs[(index + tabs.length - 1) % tabs.length];
    if (event.key === 'Home') next = tabs[0];
    if (event.key === 'End') next = tabs[tabs.length - 1];
    if (next) { event.preventDefault(); selectTab(next); next.focus(); }
  });
});
document.querySelector('.copy-code').addEventListener('click', () => {
  const activeTab = tabs.find(tab => tab.getAttribute('aria-selected') === 'true');
  const panel = document.querySelector(`#${activeTab.getAttribute('aria-controls')}`);
  copyText(panel.querySelector('code').textContent, `${activeTab.textContent} example copied.`);
});
const installPrompt = `Install Folio from https://github.com/YangTaeyoung/folio. Read INSTALL.md and
scripts/install.sh first, download the latest release for this machine, and
verify its SHA-256 against the release checksums before installing in my user
bin directory. Start folio serve with a new private persistent data directory
and localhost-only management/content listeners. Show me the private first-run
setup URL so I can create the admin account. Install the portable Folio skill
for this agent without overwriting existing files. Explain how I can create a
scoped API token and configure folio mcp; keep tokens out of logs and shell
arguments. Leave public ingress and DNS disabled until I request them.`;
document.querySelector('#copy-prompt').addEventListener('click', () => copyText(installPrompt, 'Install prompt copied. Paste it into your coding agent.'));
const navigation = [...document.querySelectorAll('.nav-link')];
function activateSection(id) {
  navigation.forEach(link => {
    const active = link.hash === `#${id}`;
    link.classList.toggle('active', active);
    if (active) link.setAttribute('aria-current', 'location');
    else link.removeAttribute('aria-current');
  });
}
if ('IntersectionObserver' in window) {
  const observer = new IntersectionObserver(entries => {
    const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => b.intersectionRatio - a.intersectionRatio);
    if (visible.length) activateSection(visible[0].target.id);
  }, { rootMargin: '-100px 0px -50% 0px', threshold: [0, .15, .5] });
  document.querySelectorAll('main>section[id]').forEach(section => observer.observe(section));
}
navigation.forEach(link => link.addEventListener('click', () => activateSection(link.hash.slice(1))));
