import { Call, Events } from '/wails/runtime.js';

const confirmation = document.getElementById('confirmation');
const presetName = document.getElementById('preset-name');

Events.On('preset:confirmation', event => {
  if (typeof event.data !== 'string' || !event.data) return;
  presetName.textContent = event.data;
  confirmation.hidden = false;
});

void Call.ByName('main.PresetConfirmation.Ready')
  .catch(error => console.error('Preset confirmation setup failed', error));
