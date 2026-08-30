// Tree-shaken icon imports from lucide
// Only import icons that are actually used in the application

// Import only the icons we need (tree-shaking works with named imports)
import { logWarn } from "@/lib/log";
import {
  AlertCircle,
  AlertTriangle,
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowRightLeft,
  ArrowUp,
  ArrowUpDown,
  AtSign,
  Bell,
  BellOff,
  Bold,
  CalendarDays,
  ChartColumn,
  Check,
  CheckCircle,
  CheckSquare,
  ChevronDown,
  ChevronRight,
  ChevronsUpDown,
  ChevronUp,
  Circle,
  CircleCheck,
  Clock,
  Code,
  Copy,
  CornerUpLeft,
  Ellipsis,
  EllipsisVertical,
  File as FileIcon,
  FileText as FileTextIcon,
  Folder,
  FolderKanban,
  FolderOpen,
  FolderPlus,
  GripVertical,
  Hash,
  Heading,
  Headphones,
  House,
  Image as ImageIcon,
  Inbox,
  Info,
  Italic,
  LayoutGrid,
  Link2,
  List,
  ListChecks,
  ListOrdered,
  ListTodo,
  Loader2,
  LogOut,
  Mail,
  Megaphone,
  MessageCircle,
  MessageSquare,
  MessageSquareText,
  Mic,
  MicOff,
  Minus,
  Monitor,
  Moon,
  PanelLeft,
  Paperclip,
  Pencil,
  Phone,
  PhoneOff,
  Pin,
  PinOff,
  PlayCircle,
  Plus,
  Quote,
  Redo2,
  Reply,
  Save,
  Search,
  Settings,
  Settings2,
  Shield,
  Smartphone,
  Smile,
  SmilePlus,
  Square as SquareIcon,
  Strikethrough,
  Sun,
  Tag,
  Trash2,
  Undo2,
  User,
  UserMinus,
  UserPlus,
  Users,
  Volume2,
  VolumeX,
  X,
} from "lucide";

type IconNode = [string, Record<string, string | number | undefined>];

// Map kebab-case names to imported icon data
const ICON_MAP: Record<string, IconNode[]> = {
  "bold": Bold,
  "italic": Italic,
  "strikethrough": Strikethrough,
  "code": Code,
  "copy": Copy,
  "heading": Heading,
  "list-ordered": ListOrdered,
  "quote": Quote,
  "undo": Undo2,
  "redo": Redo2,
  "alert-circle": AlertCircle,
  "alert-triangle": AlertTriangle,
  "arrow-down": ArrowDown,
  "arrow-left": ArrowLeft,
  "arrow-right": ArrowRight,
  "arrow-right-left": ArrowRightLeft,
  "arrow-up": ArrowUp,
  "arrow-up-down": ArrowUpDown,
  "at-sign": AtSign,
  "bell": Bell,
  "bell-off": BellOff,
  "calendar-days": CalendarDays,
  "chart-bar": ChartColumn,
  "check": Check,
  "check-circle": CheckCircle,
  "check-square": CheckSquare,
  "chevron-down": ChevronDown,
  "chevron-right": ChevronRight,
  "chevron-up": ChevronUp,
  "chevrons-up-down": ChevronsUpDown,
  "circle": Circle,
  "corner-up-left": CornerUpLeft,
  "circle-check": CircleCheck,
  "clock": Clock,
  "file": FileIcon,
  "file-text": FileTextIcon,
  "folder": Folder,
  "folder-kanban": FolderKanban,
  "folder-open": FolderOpen,
  "folder-plus": FolderPlus,
  "grip-vertical": GripVertical,
  "hash": Hash,
  "headphones": Headphones,
  "house": House,
  "image": ImageIcon,
  "inbox": Inbox,
  "info": Info,
  "layout-grid": LayoutGrid,
  "link": Link2,
  "list": List,
  "list-todo": ListTodo,
  "list-checks": ListChecks,
  "loader-2": Loader2,
  "log-out": LogOut,
  "mail": Mail,
  "megaphone": Megaphone,
  "message-circle": MessageCircle,
  "paperclip": Paperclip,
  "message-square": MessageSquare,
  "message-square-text": MessageSquareText,
  "smile": Smile,
  "smile-plus": SmilePlus,
  "mic": Mic,
  "mic-off": MicOff,
  "minus": Minus,
  "monitor": Monitor,
  "smartphone": Smartphone,
  "more-horizontal": Ellipsis,
  "more-vertical": EllipsisVertical,
  "moon": Moon,
  "panel-left": PanelLeft,
  "sun": Sun,
  "tag": Tag,
  "pencil": Pencil,
  "phone": Phone,
  "phone-off": PhoneOff,
  "pin": Pin,
  "pin-off": PinOff,
  "play-circle": PlayCircle,
  "plus": Plus,
  "reply": Reply,
  "save": Save,
  "search": Search,
  "settings": Settings,
  "settings-2": Settings2,
  "shield": Shield,
  "square": SquareIcon,
  "trash-2": Trash2,
  "user": User,
  "user-plus": UserPlus,
  "user-minus": UserMinus,
  "users": Users,
  "volume-2": Volume2,
  "volume-x": VolumeX,
  "x": X,
};

const iconCache = new Map<string, string>();

function iconNodesToSvg(nodes: IconNode[]): string {
  return nodes
    .map((node) => {
      const [tag, attrs] = node;
      const attrStr = Object.entries(attrs)
        .filter(([, v]) => v !== undefined)
        .map(([k, v]) => `${k}="${String(v).replace(/"/g, "&quot;")}"`)
        .join(" ");
      if (
        ["circle", "rect", "path", "line", "polyline", "polygon"].includes(tag)
      ) {
        return `<${tag} ${attrStr}/>`;
      }
      return `<${tag} ${attrStr}></${tag}>`;
    })
    .join("");
}

/**
 * Returns the SVG body (inner HTML) for a lucide icon.
 */
export function getIcon(name: string): string | undefined {
  if (iconCache.has(name)) {
    return iconCache.get(name);
  }

  const iconData = ICON_MAP[name];
  if (!iconData) {
    logWarn(`Icon not found: ${name}`);
    return undefined;
  }

  const svg = iconNodesToSvg(iconData);
  iconCache.set(name, svg);
  return svg;
}
