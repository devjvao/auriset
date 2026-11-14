import { Browser, Call } from '/wails/runtime.js';

const tabList = document.querySelector('.workspace-tabs');
const tabs = [...tabList.querySelectorAll('[role="tab"]')];

function selectTab(selected) {
  if (selected.getAttribute('aria-selected') === 'true') return;
  for (const tab of tabs) {
    const active = tab === selected;
    tab.setAttribute('aria-selected', String(active));
    tab.tabIndex = active ? 0 : -1;
    document.getElementById(tab.getAttribute('aria-controls')).hidden = !active;
  }
  window.scrollTo(0, 0);
}

for (const [index, tab] of tabs.entries()) {
  tab.addEventListener('click', () => selectTab(tab));
  tab.addEventListener('keydown', event => {
    const next = {
      ArrowLeft: (index + tabs.length - 1) % tabs.length,
      ArrowRight: (index + 1) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    };
    if (!Object.hasOwn(next, event.key) || event.altKey || event.ctrlKey || event.metaKey) return;
    event.preventDefault();
    const target = tabs[next[event.key]];
    selectTab(target);
    target.focus({ preventScroll: true });
  });
}

const creditsStatus = document.getElementById('credits-status');
const appVersion = document.getElementById('app-version');
void Call.ByName('main.AppInfo.Version').then(version => {
  if (typeof version !== 'string' || !version.trim()) return;
  appVersion.textContent = `Versão ${version}`;
  appVersion.hidden = false;
}).catch(error => console.error('Could not load app version', error));

async function openCredit(event) {
  event.preventDefault();
  creditsStatus.textContent = '';
  try {
    await Browser.OpenURL(event.currentTarget.href);
  } catch {
    creditsStatus.textContent = 'Não foi possível abrir o link no navegador.';
  }
}

for (const link of document.querySelectorAll('#credits-links a')) {
  link.addEventListener('click', openCredit);
  link.addEventListener('auxclick', event => {
    if (event.button === 1) void openCredit(event);
  });
}
