import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type ModalProps = {
  ariaLabel: string
  children: ReactNode
  onClose: () => void
  className?: string
}

export function Modal({ ariaLabel, children, onClose, className = '' }: ModalProps) {
  const dialogRef = useRef<HTMLElement>(null)
  const closeRef = useRef(onClose)
  closeRef.current = onClose

  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    dialogRef.current?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        closeRef.current()
        return
      }
      if (event.key !== 'Tab' || !dialogRef.current) return
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )).filter(element => !element.hasAttribute('hidden'))
      if (focusable.length === 0) {
        event.preventDefault()
        dialogRef.current.focus()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && (document.activeElement === first || !dialogRef.current.contains(document.activeElement))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (document.activeElement === last || !dialogRef.current.contains(document.activeElement))) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = previousOverflow
      previousFocus?.focus()
    }
  }, [])

  return createPortal(
    <div
      className="fixed inset-0 z-[100] grid place-items-center overflow-y-auto bg-slate-950/55 p-3 backdrop-blur-sm sm:p-6"
      onMouseDown={event => { if (event.target === event.currentTarget) closeRef.current() }}
    >
      <section
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
        tabIndex={-1}
        className={`flex max-h-[calc(100dvh-1.5rem)] min-h-0 w-full flex-col overflow-hidden rounded-2xl border border-white/70 bg-white shadow-[0_28px_90px_-24px_rgba(15,23,42,.55)] outline-none sm:max-h-[calc(100dvh-3rem)] ${className}`}
      >
        {children}
      </section>
    </div>,
    document.body,
  )
}
