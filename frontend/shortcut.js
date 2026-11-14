import { Call } from '/wails/runtime.js';

const call = (method, ...args) => Call.ByName(`main.PresetShortcut.${method}`, ...args);
const current = document.getElementById('shortcut-current');
const edit = document.getElementById('shortcut-edit');
const reset = document.getElementById('shortcut-reset');
const status = document.getElementById('shortcut-status');
const toast = document.getElementById('shortcut-toast');
const defaultShortcut = 'Ctrl+Alt+K';
const capturePrompt = 'Pressione a combinação desejada. Use Esc para cancelar.';
const toastDuration = 2500;
const modifierKeys = new Set(['Control', 'Alt', 'AltGraph', 'Shift', 'Meta']);

let shortcut;
let recording = false;
let busy = true;
let toastTimer;

function showStatus(message, error = false) {
  status.textContent = message;
  status.toggleAttribute('data-error', error);
}

function clearToast() {
  window.clearTimeout(toastTimer);
  toast.textContent = '';
}

function render() {
  current.textContent = recording ? 'Gravando…' : shortcut ?? 'Indisponível';
  if (shortcut === '' && !recording) current.textContent = 'Sem atalho';
  edit.textContent = recording ? 'Cancelar' : 'Alterar';
  edit.disabled = busy;
  reset.disabled = busy || recording;
}

function stopRecording() {
  if (!recording) return;
  recording = false;
  showStatus('');
  render();
}

function shortcutFromKey(event) {
  const key = event.key.toUpperCase();
  const modifiers = [];
  if (event.ctrlKey) modifiers.push('Ctrl');
  if (event.altKey) modifiers.push('Alt');
  if (event.shiftKey) modifiers.push('Shift');
  if (event.metaKey) modifiers.push('Win');
  if (!event.ctrlKey && !event.altKey && !event.metaKey) return '';
  if (event.getModifierState('AltGraph')) return '';

  if (/^[A-Z0-9]$/.test(key) || (/^F(?:[1-9]|1\d|2[0-4])$/.test(key) && key !== 'F12')) {
    return [...modifiers, key].join('+');
  }
  // Shift often turns number-row keys into symbols in KeyboardEvent.key.
  if (/^Digit[0-9]$/.test(event.code)) return [...modifiers, event.code.at(-1)].join('+');
  return '';
}

async function save(candidate, focusButton) {
  recording = false;
  busy = true;
  clearToast();
  showStatus('Salvando atalho…');
  render();
  try {
    shortcut = await call('Set', candidate);
    showStatus('');
    toast.textContent = 'Atalho salvo.';
    toastTimer = window.setTimeout(clearToast, toastDuration);
  } catch (error) {
    const detail = error instanceof Call.RuntimeError && error.message?.trim();
    const message = detail ? detail[0].toLocaleUpperCase('pt-BR') + detail.slice(1) : 'Não foi possível salvar o atalho';
    showStatus(message.endsWith('.') ? message : `${message}.`, true);
    try {
      shortcut = await call('Get');
    } catch {
      shortcut = undefined;
    }
  } finally {
    busy = false;
    render();
    if (document.activeElement === document.body && document.hasFocus()) {
      (focusButton.disabled ? edit : focusButton).focus();
    }
  }
}

edit.addEventListener('click', () => {
  if (recording) {
    stopRecording();
    return;
  }
  recording = true;
  showStatus(capturePrompt);
  render();
  edit.focus({ preventScroll: true });
});
edit.addEventListener('blur', stopRecording);
edit.addEventListener('keydown', event => {
  if (!recording) return;
  const unmodified = !event.ctrlKey && !event.altKey && !event.metaKey;
  if (unmodified && event.key === 'Tab') return;
  if (unmodified && !event.repeat && (event.key === 'Enter' || event.key === ' ')) return;
  event.preventDefault();
  if (event.repeat || modifierKeys.has(event.key)) return;
  if (event.key === 'Escape') {
    stopRecording();
    return;
  }
  const candidate = shortcutFromKey(event);
  if (!candidate) {
    showStatus('Use Ctrl, Alt ou Win com uma letra, um número ou uma tecla de função, exceto F12.', true);
    return;
  }
  void save(candidate, edit);
});
reset.addEventListener('click', () => void save(defaultShortcut, reset));

try {
  shortcut = await call('Get');
} catch {
  showStatus('Não foi possível consultar o atalho.', true);
} finally {
  busy = false;
  render();
}
