import type { ComponentPropsWithRef } from 'react';
import { Icon, type IconName } from './Icon';
import './components.css';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

export interface ButtonProps extends ComponentPropsWithRef<'button'> { variant?: ButtonVariant; icon?: IconName }

export function Button({ variant = 'secondary', icon, className, type = 'button', children, ...rest }: ButtonProps) {
  return (
    <button type={type} className={['btn', `btn--${variant}`, className].filter(Boolean).join(' ')} {...rest}>
      {icon && <Icon name={icon} size={16} />}
      {children}
    </button>
  );
}
