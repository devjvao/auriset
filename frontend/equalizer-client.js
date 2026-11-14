import { Call } from '/wails/runtime.js';

const call = (method, ...args) => Call.ByName(`auriset/internal/equalizer.App.${method}`, ...args);

export const equalizer = {
  presets: () => call('Presets'),
  cableStatus: () => call('CableStatus'),
  applyPreset: id => call('ApplyPreset', id),
  lastAppliedPreset: () => call('LastAppliedPreset'),
};
