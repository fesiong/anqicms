//use sessionStore
let storage: any = {};

if (
  typeof window !== 'undefined' &&
  typeof window.document !== 'undefined' &&
  typeof window.document.createElement !== 'undefined'
) {
  storage = localStorage;
}
const keyPfx = 'sh-';

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
      return null
    }
  }
  return null;
}

export function removeStore(key: string) {
  return delete storage[keyPfx + key];
}
