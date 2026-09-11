export interface FieldDef {
  key: string;
  labelKey: string;
  required?: boolean;
  type?: 'text' | 'password' | 'number' | 'boolean' | 'select';
  placeholder?: string;
  hintKey?: string;
  group?: 'basic' | 'advanced';
  options?: string[];
  showWhen?: Record<string, string[]>;
}

export interface PlatformMeta {
  label: string;
  fields: FieldDef[];
}

// CF-Connect ships DingTalk only. The other platform adapters were removed from
// this fork, so only the options the DingTalk factory actually reads are kept
// here; adding an adapter back means adding its entry back too.
export const platformMeta: Record<string, PlatformMeta> = {
  dingtalk: {
    label: 'DingTalk',
    fields: [
      { key: 'client_id', labelKey: 'fields.clientId', required: true },
      { key: 'client_secret', labelKey: 'fields.clientSecret', required: true, type: 'password' },
      { key: 'allow_from', labelKey: 'fields.allowFrom', placeholder: '* (all)', group: 'advanced' },
      { key: 'share_session_in_channel', labelKey: 'fields.sharedGroupSession', type: 'boolean', group: 'advanced' },
    ],
  },
};
