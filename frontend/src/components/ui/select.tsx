import { Children, isValidElement, type ReactElement, type ReactNode } from 'react'
import * as SelectPrimitive from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'

type OptionProps = { value?: string; disabled?: boolean; children?: ReactNode }
type SelectProps = {
  children: ReactNode
  className?: string
  name?: string
  required?: boolean
  disabled?: boolean
  value?: string
  defaultValue?: string
  id?: string
  'aria-label'?: string
  onChange?: (event: { target: { value: string } }) => void
}

export function Select({ className, children, value, defaultValue, onChange, name, required, disabled, id, 'aria-label': ariaLabel }: SelectProps) {
  const options = Children.toArray(children).filter(child => isValidElement(child) && child.type === 'option') as ReactElement<OptionProps>[]
  const placeholderOption = options.find(option => (option.props.value ?? '') === '')
  const items = options.filter(option => (option.props.value ?? '') !== '')

  return (
    <SelectPrimitive.Root
      name={name}
      required={required}
      disabled={disabled}
      value={value}
      defaultValue={defaultValue || undefined}
      onValueChange={selected => onChange?.({ target: { value: selected } })}
    >
      <SelectPrimitive.Trigger
        id={id}
        aria-label={ariaLabel}
        className={cn(
          'group flex h-10 w-full items-center justify-between gap-2 rounded-xl border border-slate-200 bg-white px-3 text-left text-sm text-slate-800 shadow-sm outline-none transition-colors hover:border-slate-300 focus:border-indigo-400 focus:ring-4 focus:ring-indigo-50 disabled:cursor-not-allowed disabled:opacity-50 data-[placeholder]:text-slate-400',
          className,
        )}
      >
        <SelectPrimitive.Value className="min-w-0 flex-1 truncate" placeholder={placeholderOption?.props.children} />
        <SelectPrimitive.Icon asChild><ChevronDown aria-hidden="true" className="size-4 shrink-0 text-slate-400 transition-transform duration-150 group-data-[state=open]:rotate-180" /></SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={6}
          align="start"
          className="z-[200] max-h-[var(--radix-select-content-available-height)] min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-xl border border-slate-200 bg-white p-1.5 text-slate-800 shadow-[0_16px_40px_-16px_rgba(15,23,42,.35)]"
        >
          <SelectPrimitive.Viewport className="max-h-[var(--radix-select-content-available-height)] overflow-y-auto">
            {items.map((option, index) => {
              const optionValue = String(option.props.value ?? '')
              return <SelectPrimitive.Item key={`${optionValue}-${index}`} value={optionValue} disabled={option.props.disabled} className="relative flex min-h-9 cursor-default select-none items-center rounded-lg py-1.5 pl-3 pr-9 text-sm outline-none data-[highlighted]:bg-indigo-50 data-[highlighted]:text-indigo-950 data-[disabled]:pointer-events-none data-[disabled]:opacity-40">
                <SelectPrimitive.ItemText>{option.props.children}</SelectPrimitive.ItemText>
                <SelectPrimitive.ItemIndicator className="absolute right-3 inline-flex items-center"><Check className="size-4 text-indigo-600" /></SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            })}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  )
}
