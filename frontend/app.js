import { Call, Events } from '/wails/runtime.js';
import { equalizer } from './equalizer-client.js';
import { createResponseChart, formatGain } from './response-chart.js';

const element = id => document.getElementById(id);

const ui = {
  groups: element('preset-groups'),
  presetFeedback: element('preset-feedback'),
  title: element('preset-title'),
  description: element('preset-description'),
  preamp: element('preamp'),
  preampValue: element('preamp-value'),
  connection: element('connection'),
  connectionLabel: element('connection-label'),
  cableFeedback: element('cable-feedback'),
  apply: element('apply-button'),
  lastApplied: element('last-applied'),
  writeFeedback: element('write-feedback'),
  confirmation: element('apply-confirmation'),
  confirmationName: element('apply-confirmation-name'),
  chart: element('response-chart'),
  referenceLegend: element('reference-legend'),
  referenceName: element('reference-name'),
  reading: element('chart-reading'),
};

const chart = createResponseChart(ui.chart, ui.reading);

const connectionLabels = {
  checking: 'Verificando cabo',
  connected: 'Conectado',
  disconnected: 'Desconectado',
  multiple: 'Vários cabos',
  error: 'Falha na detecção',
};
const groupLabels = new Map([
  ['preference', 'Curvas de preferência'],
  ['reference', 'Curvas de referência'],
]);

function applyErrorDetail(error) {
  if (!(error instanceof Call.RuntimeError) || !error.message) {
    return 'Tente novamente.';
  }
  const message = error.message.charAt(0).toLocaleUpperCase('pt-BR') + error.message.slice(1);
  return message.endsWith('.') ? message : `${message}.`;
}

function setupTrayNotice() {
  const notice = element('tray-notice');
  const confirm = element('tray-notice-confirm');
  const feedback = element('tray-notice-error');

  notice.addEventListener('cancel', event => event.preventDefault());
  confirm.addEventListener('click', async () => {
    if (confirm.disabled) return;
    confirm.disabled = true;
    feedback.hidden = true;
    try {
      await Call.ByName('main.TrayNotice.Dismiss');
      notice.close();
    } catch {
      feedback.hidden = false;
    } finally {
      confirm.disabled = false;
      if (notice.open) confirm.focus();
    }
  });
  Events.On('tray:notice', () => {
    if (!notice.open) notice.showModal();
  });
  Call.ByName('main.TrayNotice.Ready').catch(error => console.error('Tray notice setup failed', error));
}

let catalogue = [];
let selected;
let cable = { state: 'checking' };
let statusRequest;
let writing = false;
let lastAppliedRevision = 0;
let lastAppliedLoaded = false;
let confirmationTimer;

async function refreshLastApplied() {
  const revision = ++lastAppliedRevision;
  try {
    const preset = await equalizer.lastAppliedPreset();
    if (revision !== lastAppliedRevision) return;
    ui.lastApplied.textContent = preset || 'Nenhum';
    lastAppliedLoaded = true;
  } catch {
    if (revision === lastAppliedRevision && !lastAppliedLoaded) {
      ui.lastApplied.textContent = 'Indisponível';
    }
  }
}

Events.On('preset:sent', refreshLastApplied);
void refreshLastApplied();

function updateApplyButton() {
  ui.apply.disabled = writing || !selected || cable.state !== 'connected';
  ui.apply.textContent = writing ? 'Enviando…' : 'Aplicar preset';
}

function selectPreset(preset) {
  selected = preset;
  ui.title.textContent = preset.name;
  ui.description.textContent = preset.description;
  ui.preampValue.textContent = formatGain(preset.preampDb);
  ui.preamp.hidden = false;
  ui.referenceName.textContent = `Alvo ${preset.targetName}`;
  ui.referenceLegend.hidden = false;
  chart.setPreset(preset);
  updateApplyButton();
}

function renderCatalogue() {
  for (const [group, name] of groupLabels) {
    const fieldset = document.createElement('fieldset');
    fieldset.className = 'preset-group';
    const legend = document.createElement('legend');
    legend.textContent = name;
    fieldset.append(legend);

    const options = document.createElement('div');
    options.className = 'preset-options';
    for (const preset of catalogue.filter(item => item.group === group)) {
      const row = document.createElement('label');
      row.className = 'preset-row';
      const radio = document.createElement('input');
      radio.type = 'radio';
      radio.name = 'preset';
      radio.value = preset.id;
      radio.checked = preset === catalogue[0];
      radio.addEventListener('change', () => selectPreset(preset));
      const label = document.createElement('span');
      label.className = 'preset-label';
      label.textContent = preset.name;
      row.append(radio, label);
      options.append(row);
    }
    fieldset.append(options);
    ui.groups.append(fieldset);
  }
  ui.presetFeedback.hidden = true;
  ui.groups.hidden = false;
  selectPreset(catalogue[0]);
}

function setCableStatus(status) {
  if (cable.state !== status.state) {
    ui.connection.dataset.state = status.state;
    ui.connectionLabel.textContent = connectionLabels[status.state] ?? connectionLabels.error;
  }
  cable = status;
  const message = {
    disconnected: status.message || 'Conecte o cabo do Auri para aplicar um preset.',
    multiple: status.message || 'Conecte apenas um cabo do Auri para aplicar um preset.',
    error: status.message || 'Não foi possível detectar o cabo do Auri.',
  }[status.state] ?? '';
  ui.cableFeedback.textContent = message;
  ui.cableFeedback.hidden = !message;
  ui.cableFeedback.toggleAttribute('data-error', status.state === 'error');
  updateApplyButton();
}

function refreshStatus() {
  if (writing || statusRequest) return statusRequest;
  statusRequest = equalizer.cableStatus()
    .then(setCableStatus)
    .catch(() => setCableStatus({ state: 'error', message: 'Não foi possível verificar a conexão do cabo.' }))
    .finally(() => { statusRequest = undefined; });
  return statusRequest;
}

function isMainWindowActive() {
  return document.visibilityState === 'visible' && document.hasFocus();
}

function refreshOnReturn() {
  if (!isMainWindowActive()) return;
  void refreshStatus();
  void refreshLastApplied();
}

function setWriteFeedback(message, error = false) {
  ui.writeFeedback.textContent = message;
  ui.writeFeedback.hidden = !message;
  ui.writeFeedback.toggleAttribute('data-error', error);
}

function hideApplyConfirmation() {
  window.clearTimeout(confirmationTimer);
  ui.confirmation.hidden = true;
}

function showApplyConfirmation(name) {
  ui.confirmationName.textContent = name;
  ui.confirmation.hidden = false;
  confirmationTimer = window.setTimeout(hideApplyConfirmation, 2200);
}

async function applySelected() {
  if (writing || !selected || cable.state !== 'connected') return;
  const preset = selected;
  writing = true;
  updateApplyButton();
  setWriteFeedback('');
  hideApplyConfirmation();
  try {
    await statusRequest;
    if (cable.state !== 'connected') return;
    const result = await equalizer.applyPreset(preset.id);
    void refreshLastApplied();
    if (result.historySaved === false) {
      setWriteFeedback('Não foi possível salvar o histórico.', true);
    }
    showApplyConfirmation(result.name);
  } catch (error) {
    setWriteFeedback(`Não foi possível enviar o preset ${preset.name}. ${applyErrorDetail(error)}`, true);
  } finally {
    writing = false;
    updateApplyButton();
    if (isMainWindowActive()) await refreshStatus();
  }
}

setupTrayNotice();
ui.apply.addEventListener('click', applySelected);
window.addEventListener('focus', refreshOnReturn);
document.addEventListener('visibilitychange', refreshOnReturn);
if (isMainWindowActive()) void refreshStatus();
window.setInterval(() => {
  if (isMainWindowActive()) void refreshStatus();
}, 1500);
try {
  catalogue = await equalizer.presets();
  if (!catalogue?.length) throw new Error('Nenhum preset disponível.');
  chart.setPresets(catalogue);
  renderCatalogue();
} catch {
  ui.presetFeedback.textContent = 'Não foi possível carregar os presets.';
}
