import type { DeepReadonly } from 'vue'
import type { Collection, Target } from './data'
import { validateCollectionForm } from './form'
export type SourceDraft = { key: number; id: string | null; watch_target_id: string; name: string; url: string; instruction: string; error: string; errorField: 'name' | 'url' | 'instruction' | '' }
let draftKey = 0
export const newSource = (targetID: string): SourceDraft => ({ key: ++draftKey, id: null, watch_target_id: targetID, name: '', url: '', instruction: '', error: '', errorField: '' })
export const sourceDrafts = (target: DeepReadonly<Target>): SourceDraft[] => target.sources.map(source => ({ ...newSource(target.id), ...source }))
export const sourceChanges = (drafts: SourceDraft[], collection: DeepReadonly<Collection>) => drafts.filter(draft => {
  const previous = collection.targets.find(target => target.id === draft.watch_target_id)?.sources.find(source => source.id === draft.id)
  return !previous || previous.name !== draft.name || previous.url !== draft.url || previous.instruction !== draft.instruction
}).map(({ id, watch_target_id, name, url, instruction }) => ({ id, watch_target_id, name, url, instruction }))
export const validSourceURL = (value: string) => {
  if ([...value].length > 2048 || /[\p{White_Space}\p{Cc}\\]/u.test(value) || /%(?![0-9a-fA-F]{2})/.test(value)) { return false }
  const match = /^https?:\/\/([^/?#]+)/i.exec(value)
  if (!match || /[@%]/.test(match[1])) { return false }
  const authority = match[1]
  const parts = /^(\[[0-9a-fA-F:.]+\]|[a-zA-Z0-9.-]+)(?::([0-9]+))?$/.exec(authority)
  if (!parts || parts[2] !== undefined && (Number(parts[2]) < 1 || Number(parts[2]) > 65535)) { return false }
  if (!parts[1].startsWith('[') && (parts[1].length > 253 || parts[1].replace(/\.$/, '').split('.').some(label => !/^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$/.test(label)))) { return false }
  if (parts[1].startsWith('[')) {
    try { new URL(value) } catch { return false }
  }
  return true
}
export const validateSources = (drafts: SourceDraft[]) => {
  let valid = true
  for (const draft of drafts) {
    const fields = { name: '', instruction: '', days: '' }
    validateCollectionForm({ name: draft.name, instruction: draft.instruction, mode: 'ai', days: '' }, fields)
    draft.errorField = fields.name ? 'name' : !validSourceURL(draft.url) ? 'url' : fields.instruction ? 'instruction' : ''
    draft.error = [fields.name.replace('名前', '表示名'), fields.instruction.replace('調査方針', '補足指示'), validSourceURL(draft.url) ? '' : 'URLは2,048文字以内のhttp://またはhttps://で始まるURLを入力してください。空白や認証情報は含められません。'].filter(Boolean).join(' ')
    if (draft.error) { valid = false }
  }
  return valid
}
