import type { DeepReadonly } from 'vue'
import type { Collection } from './data'
import { validateCollectionForm } from './form'

export type TargetDraft = { id: string | null; name: string; instruction: string; error: string }
export const targetDrafts = (collection: DeepReadonly<Collection>): TargetDraft[] => collection.targets.map(target => ({ id: target.id, name: target.name, instruction: target.instruction, error: '' }))
export const targetChanges = (drafts: TargetDraft[], collection: DeepReadonly<Collection>) => drafts.filter(draft => {
  const previous = collection.targets.find(target => target.id === draft.id)
  return !previous || previous.name !== draft.name || previous.instruction !== draft.instruction
}).map(({ id, name, instruction }) => ({ id, name, instruction }))
export const validateTargets = (drafts: TargetDraft[]) => {
  let valid = true
  for (const draft of drafts) {
    const fields = { name: '', instruction: '', days: '' }
    if (!validateCollectionForm({ name: draft.name, instruction: draft.instruction, mode: 'ai', days: '' }, fields)) {
      valid = false
    }
    draft.error = [fields.name, fields.instruction.replace('調査方針', '補足指示')].filter(Boolean).join(' ')
  }
  return valid
}
