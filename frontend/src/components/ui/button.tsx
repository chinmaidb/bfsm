import { cva, type VariantProps } from 'class-variance-authority'
import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

const variants = cva('inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-xl text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-indigo-100 disabled:pointer-events-none disabled:opacity-50', {
  variants: {
    variant: {
      default: 'bg-indigo-600 text-white shadow-sm hover:bg-indigo-700',
      secondary: 'bg-slate-100 text-slate-700 hover:bg-slate-200',
      outline: 'border border-slate-200 bg-white text-slate-700 hover:bg-slate-50',
      ghost: 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
      danger: 'bg-rose-50 text-rose-700 hover:bg-rose-100',
    },
    size: { default: 'h-10 px-4', sm: 'h-8 rounded-lg px-3 text-xs', icon: 'size-10' },
  },
  defaultVariants: { variant: 'default', size: 'default' },
})

type Props = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof variants>
export function Button({ className, variant, size, ...props }: Props) {
  return <button className={cn(variants({ variant, size }), className)} {...props} />
}
