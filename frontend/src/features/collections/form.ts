export type CollectionForm = { name: string; instruction: string; mode: string; days: string }
export type CollectionFields = { name: string; instruction: string; days: string }
export const validateCollectionForm = (form: CollectionForm, fields: CollectionFields) => {
  fields.name = /^\p{White_Space}*$/u.test(form.name) || [...form.name].length > 200 || form.name.includes('\0') ? '名前は空白だけにせず、1〜200文字で入力してください。' : ''
  fields.instruction = [...form.instruction].length > 10000 || form.instruction.includes('\0') ? '調査方針は10,000文字以内で入力してください。' : ''
  const days = Number(form.days)
  fields.days = form.mode !== 'ai' && (form.mode !== 'days' || !/^\d+$/.test(form.days) || !Number.isInteger(days) || days < 1 || days > 365) ? '日数は1〜365の整数で入力してください。' : ''
  return !fields.name && !fields.instruction && !fields.days
}
