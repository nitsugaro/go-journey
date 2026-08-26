import type { ButtonHTMLAttributes, ReactNode } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'warning' | 'success' | 'status'
type ButtonSize = 'xs' | 'sm' | 'md' | 'lg'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode
  variant?: ButtonVariant
  size?: ButtonSize
  active?: boolean
  iconOnly?: boolean
}

const base =
  'inline-flex shrink-0 items-center justify-center gap-2 border font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-blue)] focus-visible:ring-offset-0 disabled:cursor-not-allowed disabled:border-[var(--color-border-soft)] disabled:bg-[var(--color-surface-soft)] disabled:text-[var(--color-disabled)] disabled:shadow-none'

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    'border-[var(--color-blue)] bg-[var(--color-blue)] text-[var(--color-white)] shadow-sm shadow-[var(--color-muted-faint)] hover:brightness-105',
  secondary:
    'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-ink)] hover:border-[var(--color-blue-border)] hover:bg-[var(--color-blue-soft)] hover:text-[var(--color-blue)]',
  ghost:
    'border-transparent bg-transparent text-[var(--color-muted)] hover:bg-[var(--color-surface-soft)] hover:text-[var(--color-blue)]',
  danger:
    'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-red)] hover:border-[var(--color-red-border)] hover:bg-[var(--color-red-soft)]',
  warning:
    'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-warning)] hover:border-[var(--color-warning-border)] hover:bg-[var(--color-warning-soft)]',
  success:
    'border-[var(--color-border-soft)] bg-[var(--color-surface-soft)] text-[var(--color-green)] hover:border-[var(--color-green-border)] hover:bg-[var(--color-green-soft)]',
  status:
    'border-[var(--color-border-soft)] bg-[var(--color-surface-subtle)] text-[var(--color-muted)]',
}

const activeClass =
  'border-[var(--color-blue)] bg-[var(--color-blue)] text-[var(--color-white)] shadow-sm shadow-[var(--color-muted-faint)]'

const sizeClasses: Record<ButtonSize, string> = {
  xs: 'h-7 rounded-lg px-2 text-xs',
  sm: 'h-8 rounded-xl px-3 text-xs',
  md: 'h-10 rounded-xl px-4 text-sm',
  lg: 'h-11 rounded-2xl px-5 text-sm',
}

const iconSizeClasses: Record<ButtonSize, string> = {
  xs: 'h-7 w-7 rounded-lg p-0 text-xs',
  sm: 'h-8 w-8 rounded-xl p-0 text-sm',
  md: 'h-9 w-9 rounded-xl p-0 text-base',
  lg: 'h-10 w-10 rounded-xl p-0 text-lg',
}

export function Button({
  children,
  variant = 'secondary',
  size = 'md',
  active = false,
  iconOnly = false,
  className = '',
  type = 'button',
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={[
        base,
        active ? activeClass : variantClasses[variant],
        iconOnly ? iconSizeClasses[size] : sizeClasses[size],
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}

type IconButtonProps = Omit<ButtonProps, 'children' | 'iconOnly'> & {
  label: string
  children: ReactNode
}

export function IconButton({ label, children, title, ...props }: IconButtonProps) {
  return (
    <Button iconOnly aria-label={label} title={title || label} {...props}>
      {children}
    </Button>
  )
}
