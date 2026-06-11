import type { ComponentProps, ReactNode } from 'react';
import { Button, Card, Chip, Spinner, Skeleton } from '@heroui/react';
import { CheckCircle, Clock, XCircle, AlertTriangle } from 'lucide-react';

const buttonTone = {
  primary: 'bg-purple-600 text-white hover:bg-purple-500',
  neutral: 'bg-gray-800 text-gray-100 hover:bg-gray-700 border border-gray-700',
  subtle: 'bg-transparent text-gray-400 hover:text-gray-100 hover:bg-gray-800',
  success: 'bg-green-500/10 text-green-300 hover:bg-green-500/20',
  danger: 'bg-red-500/10 text-red-300 hover:bg-red-500/20',
};

type ShadowButtonProps = ComponentProps<typeof Button> & {
  tone?: keyof typeof buttonTone;
};

export function ShadowButton({ tone = 'neutral', className = '', ...props }: ShadowButtonProps) {
  return (
    <Button
      {...props}
      className={`min-h-9 rounded-lg px-3 text-sm font-medium transition-colors ${buttonTone[tone]} ${className}`}
    />
  );
}

type IconButtonProps = ShadowButtonProps & {
  label: string;
};

export function IconButton({ label, className = '', ...props }: IconButtonProps) {
  return (
    <ShadowButton
      {...props}
      isIconOnly
      aria-label={label}
      className={`h-8 w-8 min-w-8 p-0 ${className}`}
    />
  );
}

export function ShadowCard({ className = '', ...props }: ComponentProps<typeof Card>) {
  return (
    <Card
      {...props}
      className={`rounded-lg border border-gray-800 bg-gray-900 text-gray-100 shadow-none ${className}`}
    />
  );
}

export function SectionCard({
  title,
  description,
  icon,
  children,
  className = '',
}: {
  title: string;
  description?: string;
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <ShadowCard className={className}>
      <Card.Header className="flex items-start gap-3 border-b border-gray-800 px-5 py-4">
        {icon && <div className="mt-0.5 text-purple-300">{icon}</div>}
        <div>
          <Card.Title className="text-base font-semibold text-gray-100">{title}</Card.Title>
          {description && <Card.Description className="mt-1 text-sm text-gray-500">{description}</Card.Description>}
        </div>
      </Card.Header>
      <Card.Content className="px-5 py-4">{children}</Card.Content>
    </ShadowCard>
  );
}

const statusTone: Record<string, { cls: string; icon: typeof CheckCircle; label: string }> = {
  active: { cls: 'bg-green-500/10 text-green-300 border-green-500/20', icon: CheckCircle, label: 'active' },
  candidate: { cls: 'bg-yellow-500/10 text-yellow-300 border-yellow-500/20', icon: Clock, label: 'candidate' },
  disabled: { cls: 'bg-gray-700/60 text-gray-300 border-gray-700', icon: XCircle, label: 'disabled' },
  conflicted: { cls: 'bg-red-500/10 text-red-300 border-red-500/20', icon: AlertTriangle, label: 'conflicted' },
};

export function StatusChip({ status }: { status: string }) {
  const config = statusTone[status] ?? statusTone.candidate;
  const Icon = config.icon;
  return (
    <Chip className={`gap-1 rounded-md border px-2 py-0.5 text-xs ${config.cls}`}>
      <Icon size={12} />
      <span>{config.label}</span>
    </Chip>
  );
}

export function TagChip({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <Chip className={`rounded-md border border-gray-700 bg-gray-800 px-2 py-0.5 text-xs text-gray-300 ${className}`}>
      {children}
    </Chip>
  );
}

export function LoadingState({ label = 'Loading...' }: { label?: string }) {
  return (
    <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-gray-500">
      <Spinner className="text-purple-300" />
      <span className="text-sm">{label}</span>
    </div>
  );
}

export function SkeletonRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, index) => (
        <Skeleton key={index} className="h-20 rounded-lg bg-gray-900" />
      ))}
    </div>
  );
}
