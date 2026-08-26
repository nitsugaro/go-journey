import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  CheckCircle2,
  Copy,
  Download,
  FilePenLine,
  FileText,
  LoaderCircle,
  Pencil,
  Plus,
  Save,
  Trash2,
  Upload,
  X,
} from 'lucide-react'

type IconProps = {
  size?: number
}

export function PlusIcon({ size = 20 }: IconProps) {
  return <Plus size={size} strokeWidth={2.4} aria-hidden="true" />
}

export function SavedIcon({ size = 18 }: IconProps) {
  return <CheckCircle2 size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function SaveIcon({ size = 18 }: IconProps) {
  return <Save size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function DeleteIcon({ size = 18 }: IconProps) {
  return <Trash2 size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function CopyIcon({ size = 18 }: IconProps) {
  return <Copy size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function ExportIcon({ size = 18 }: IconProps) {
  return <Download size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function ImportIcon({ size = 18 }: IconProps) {
  return <Upload size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function ChangesIcon({ size = 18 }: IconProps) {
  return <FilePenLine size={size} strokeWidth={2.1} aria-hidden="true" />
}

export function SpinnerIcon({ size = 20 }: IconProps) {
  return <LoaderCircle size={size} strokeWidth={2.4} aria-hidden="true" className="animate-spin" />
}

export function PlusMiniIcon() {
  return <Plus size={16} strokeWidth={2.2} aria-hidden="true" />
}

export function EditMiniIcon() {
  return <Pencil size={16} strokeWidth={2} aria-hidden="true" />
}

export function CloseMiniIcon() {
  return <X size={16} strokeWidth={2.2} aria-hidden="true" />
}

export function BackMiniIcon() {
  return <ArrowLeft size={16} strokeWidth={2.2} aria-hidden="true" />
}

export function ArrowUpMiniIcon() {
  return <ArrowUp size={16} strokeWidth={2.1} aria-hidden="true" />
}

export function ArrowDownMiniIcon() {
  return <ArrowDown size={16} strokeWidth={2.1} aria-hidden="true" />
}

export function NoteTinyIcon() {
  return <FileText size={14} strokeWidth={2} aria-hidden="true" />
}
