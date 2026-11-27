import { LinkMenuItem, type AiEditor } from 'aieditor';

export function enableRelativeLinks(editor: AiEditor) {
  for (const item of [
    ...editor.getMenuItems(),
    ...editor.getBubbleMenuItems(),
  ]) {
    if (!(item instanceof LinkMenuItem)) continue;
    const button = item.getElement();
    const dialogId = button?.getAttribute('aria-controls');
    const dialog = dialogId
      ? button?.ownerDocument.getElementById(dialogId)
      : null;
    const input = dialog?.querySelector<HTMLInputElement>('input[type="url"]');
    // Native URL inputs reject relative references; setLink still validates protocols.
    if (input) input.type = 'text';
  }
}
