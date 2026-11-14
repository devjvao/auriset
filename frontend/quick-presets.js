import { Call, Events, Window } from '/wails/runtime.js';
import { equalizer } from './equalizer-client.js';

const list = document.getElementById('preset-list');
const feedback = document.getElementById('feedback');
const cableStatus = document.getElementById('cable-status');
const cableLabels = {
  checking: 'Verificando cabo',
  connected: 'Cabo conectado',
  disconnected: 'Cabo desconectado',
  multiple: 'Vários cabos',
  error: 'Falha na detecção',
};
const groupLabels = [
  ['preference', 'Curvas de preferência'],
  ['reference', 'Curvas de referência'],
];

const options = [];
let focusedIndex = 0;
let cable = { state: 'checking' };
let presetsRequest;
let cableRequest;
let writing = false;
let closing = false;
let opening = 0;
let result;

function render() {
  if (cableStatus.dataset.state !== cable.state) {
    cableStatus.dataset.state = cable.state;
    cableStatus.textContent = cableLabels[cable.state] ?? cableLabels.error;
  }
  const disabled = String(writing || closing || cable.state !== 'connected');
  for (const option of options) {
    if (option.getAttribute('aria-disabled') !== disabled) option.setAttribute('aria-disabled', disabled);
  }

  const status = result ?? (cable.state !== 'connected' && cable.message ? { message: cable.message } : null);
  const message = status?.message ?? (options.length ? '' : 'Carregando presets…');
  // Avoid announcing identical status text every time the cable is checked.
  if (feedback.textContent !== message) feedback.textContent = message;
  feedback.dataset.tone = status?.tone ?? '';
}

function focusOption(index) {
  if (!options[index]) return;
  options[focusedIndex].tabIndex = -1;
  focusedIndex = index;
  options[index].tabIndex = 0;
  options[index].focus();
}

function renderPresets(presets) {
  const groups = groupLabels.map(([id, label]) => {
    const section = document.createElement('section');
    const heading = document.createElement('h2');
    const rows = document.createElement('ul');
    section.className = 'preset-group';
    heading.id = `preset-group-${id}`;
    heading.textContent = label;
    section.setAttribute('aria-labelledby', heading.id);
    section.append(heading, rows);

    for (const preset of presets.filter(item => item.group === id)) {
      const row = document.createElement('li');
      const button = document.createElement('button');
      const index = options.length;
      button.type = 'button';
      button.className = 'preset-option';
      button.textContent = preset.name;
      button.tabIndex = index === 0 ? 0 : -1;
      button.addEventListener('focus', () => {
        options[focusedIndex].tabIndex = -1;
        focusedIndex = index;
        button.tabIndex = 0;
      });
      button.addEventListener('click', () => void applyPreset(preset));
      options.push(button);
      row.append(button);
      rows.append(row);
    }
    return section;
  });
  if (!options.length) throw new Error('No presets returned');
  list.append(...groups);
}

function loadPresets() {
  if (options.length || presetsRequest) return presetsRequest;
  presetsRequest = equalizer.presets().then(renderPresets).catch(() => {
    result = { tone: 'error', message: 'Não foi possível carregar os presets. Reabra a janela para tentar novamente.' };
  }).finally(() => {
    presetsRequest = undefined;
    render();
  });
  return presetsRequest;
}

function refreshCable() {
  if (cableRequest) return cableRequest;
  cableRequest = equalizer.cableStatus().then(status => {
    cable = status;
  }).catch(() => {
    cable = { state: 'error', message: 'Não foi possível verificar a conexão do cabo.' };
  }).finally(() => {
    cableRequest = undefined;
    render();
  });
  return cableRequest;
}

async function onOpen() {
  opening++;
  closing = false;
  if (!writing) result = undefined;
  cable = { state: 'checking' };
  render();
  focusOption(focusedIndex);
  await Promise.all([loadPresets(), refreshCable()]);
  if (!closing && document.activeElement === document.body) focusOption(focusedIndex);
}

async function closePicker(presetName) {
  if (closing && !presetName) return;
  closing = true;
  render();
  try {
    if (presetName) {
      await Call.ByName('main.PresetConfirmation.ShowAfterPicker', presetName);
    } else {
      await Window.Close();
    }
  } catch {
    closing = false;
    result = { tone: 'error', message: 'Não foi possível fechar a janela. Tente novamente.' };
    render();
  }
}

async function applyPreset(preset) {
  if (writing || closing || cable.state !== 'connected') return;
  const startedOn = opening;
  writing = true;
  result = { message: `Enviando ${preset.name} ao cabo…` };
  render();
  try {
    const { name } = await equalizer.applyPreset(preset.id);
    result = { tone: 'success', message: `Preset ${name} enviado ao cabo.` };
    void Events.Emit('preset:sent', name).catch(error => console.error('Could not share the preset result', error));
    if (opening === startedOn) await closePicker(name);
  } catch (error) {
    const detail = error instanceof Call.RuntimeError && error.message?.trim();
    const message = detail ? detail[0].toLocaleUpperCase('pt-BR') + detail.slice(1) : 'Não foi possível enviar o preset';
    result = { tone: 'error', message: message.endsWith('.') ? message : `${message}.` };
  } finally {
    writing = false;
    render();
    if (!closing) void refreshCable();
  }
}

document.addEventListener('keydown', event => {
  if (event.defaultPrevented) return;
  if (event.key === 'Escape') {
    event.preventDefault();
    if (!event.repeat) void closePicker();
    return;
  }
  if (!options.includes(event.target)) return;
  const navigation = {
    ArrowDown: Math.min(focusedIndex + 1, options.length - 1),
    ArrowUp: Math.max(focusedIndex - 1, 0),
    Home: 0,
    End: options.length - 1,
  };
  if (Object.hasOwn(navigation, event.key)) {
    event.preventDefault();
    focusOption(navigation[event.key]);
  } else if (event.key === 'Enter' && event.repeat) {
    event.preventDefault();
  }
});

Events.On('quick-presets:opened', onOpen);
void onOpen();
window.setInterval(() => {
  if (document.hasFocus() && !writing && !closing) void refreshCable();
}, 1500);
