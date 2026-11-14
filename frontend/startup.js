import { Call } from '/wails/runtime.js';

const call = (method, ...args) => Call.ByName(`main.WindowsStartup.${method}`, ...args);
const setting = document.getElementById('windows-startup');
const status = document.getElementById('startup-status');

let enabled = false;

async function loadSetting() {
  try {
    enabled = await call('Enabled');
    setting.checked = enabled;
    setting.disabled = false;
  } catch {
    status.textContent = 'Não foi possível consultar a inicialização com o Windows.';
  }
}

setting.addEventListener('change', async () => {
  const requested = setting.checked;
  setting.disabled = true;
  status.textContent = '';
  try {
    await call('SetEnabled', requested);
    enabled = requested;
  } catch {
    setting.checked = enabled;
    status.textContent = 'Não foi possível salvar a configuração de inicialização.';
  } finally {
    setting.disabled = false;
  }
});

void loadSetting();
