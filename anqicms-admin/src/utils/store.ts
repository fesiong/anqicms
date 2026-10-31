//use sessionStore
let storage: any = {};

if (typeof localStorage !== 'undefined') {
  storage = localStorage;
}
const keyPfx = 'anqi-';

export function setStore(key: string, value: any) {
  let data = JSON.stringify(value);

  return (storage[keyPfx + key] = data);
}

export function getStore(key: string) {
  let data = storage[keyPfx + key];
  if (data) {
    try {
      return JSON.parse(data);
    } catch (e) {
      return null;
    }
  }
  return null;
}

export function removeStore(key: string) {
  return delete storage[keyPfx + key];
}

export function setSessionStore(key: string, value: any) {
  let data = JSON.stringify(value);

  return (sessionStorage[keyPfx + key] = data);
}

export function getSessionStore(key: string) {
  let data = sessionStorage[keyPfx + key];
  if (data) {
    try {
      return JSON.parse(data);
    } catch (e) {
      return null;
    }
  }
  return null;
}
