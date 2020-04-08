import { range } from './Utils/helper';

export const AXES_CONFIG = {
  effectiveTemperature: {
    start: 10000,
    end: 30000,
    total: 100,
    step: 200
  },
  surfaceGravity: {
    start: 1,
    end: 5,
    total: 20,
    step: 0.2
  },
  microturbulance: {
    start: 0,
    end: 4,
    step: 1,
    total: 5
  },
  inclination: {
    start: 5,
    end: 90,
    step: 5,
    total: 10
  }
};

export const effectiveTemperatureAxes = range(AXES_CONFIG.effectiveTemperature);
export const surfaceGravityAxes = range(AXES_CONFIG.surfaceGravity);
export const microturbulanceAxes = range(AXES_CONFIG.microturbulance);
