export type EmailTemplate = {
  open?: boolean;
  key: string;
  type: string;
  name: string;
  description: string;
  delay?: number;
  subject?: string;
  content?: string;
};
