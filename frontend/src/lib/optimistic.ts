import type { Dispatch, SetStateAction } from 'react'
import type { Row } from '../screens'

export type SetRows = Dispatch<SetStateAction<Row[]>>
export type PendingRow = Row & { _optimistic?: boolean }

export async function optimisticCreate(
  setRows: SetRows,
  draft: Row,
  request: () => Promise<Row>,
) {
  const temporaryID = `pending-${crypto.randomUUID()}`
  const pending = { ...draft, id: temporaryID, _optimistic: true } as PendingRow
  setRows(current => [pending, ...current])
  try {
    const saved = await request()
    setRows(current => current.map(row => String(row.id) === temporaryID ? saved : row))
    return saved
  } catch (error) {
    setRows(current => current.filter(row => String(row.id) !== temporaryID))
    throw error
  }
}

export async function optimisticUpdate(
  setRows: SetRows,
  currentRows: Row[],
  id: string,
  changes: Row,
  request: () => Promise<Row>,
) {
  const previous = currentRows.find(row => String(row.id) === id)
  setRows(current => current.map(row => String(row.id) === id ? { ...row, ...changes, _optimistic: true } : row))
  try {
    const saved = await request()
    setRows(current => current.map(row => String(row.id) === id ? saved : row))
    return saved
  } catch (error) {
    if (previous) setRows(current => current.map(row => String(row.id) === id ? previous : row))
    throw error
  }
}

export async function optimisticRemove(
  setRows: SetRows,
  currentRows: Row[],
  id: string,
  request: () => Promise<unknown>,
) {
  const index = currentRows.findIndex(row => String(row.id) === id)
  const previous = currentRows[index]
  setRows(current => current.filter(row => String(row.id) !== id))
  try {
    await request()
  } catch (error) {
    if (previous) setRows(current => current.some(row => String(row.id) === id)
      ? current
      : [...current.slice(0, index), previous, ...current.slice(index)])
    throw error
  }
}
