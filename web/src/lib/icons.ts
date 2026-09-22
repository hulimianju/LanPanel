/**
 * 图标库：只按需引入精选的 lucide 图标，避免把 1500+ 图标全部打包（路由器上体积敏感）。
 * 键名为 kebab-case，存储在卡片的 icon.value 中。
 */
import {
  Activity, AppWindow, Archive, BookOpen, Bot, Box, Boxes, Braces, Calendar, Camera, ChartLine, Cloud,
  CloudDownload, Code, Cpu, Database, Download, Film, Folder, FolderOpen, Gamepad2, GitBranch, Github, Globe,
  HardDrive, Headphones, House, Image, Key, Laptop, Layers, LayoutGrid, Lightbulb, Link, Lock, Mail,
  MessageCircle, Monitor, Music, Network, Newspaper, NotebookPen, Package, Palette, Phone, Play, Printer,
  Radio, Router, Rss, Search, Server, Settings, Shield, ShieldCheck, ShoppingCart, Smartphone, Speaker,
  SquareTerminal, Star, Thermometer, Tv, Users, Video, Wifi, Wrench, Zap, Bookmark, Clock, Webcam, Cast,
  Container, Workflow, Gauge, ScanSearch, FileText, Bell, Car, Plane, Leaf, Sun, Moon,
} from 'lucide-vue-next'
import type { Component } from 'vue'

export const ICONS: Record<string, Component> = {
  'house': House, 'layout-grid': LayoutGrid, 'bookmark': Bookmark, 'star': Star, 'globe': Globe, 'link': Link,
  'server': Server, 'hard-drive': HardDrive, 'database': Database, 'layers': Layers, 'cpu': Cpu, 'container': Container,
  'box': Box, 'boxes': Boxes, 'package': Package, 'network': Network, 'router': Router, 'wifi': Wifi,
  'shield': Shield, 'shield-check': ShieldCheck, 'lock': Lock, 'key': Key, 'monitor': Monitor, 'laptop': Laptop,
  'smartphone': Smartphone, 'tv': Tv, 'cast': Cast, 'printer': Printer, 'camera': Camera, 'webcam': Webcam,
  'speaker': Speaker, 'film': Film, 'play': Play, 'video': Video, 'music': Music, 'headphones': Headphones,
  'radio': Radio, 'image': Image, 'gamepad': Gamepad2, 'download': Download, 'cloud-download': CloudDownload,
  'cloud': Cloud, 'folder': Folder, 'folder-open': FolderOpen, 'archive': Archive, 'file-text': FileText,
  'chart': ChartLine, 'activity': Activity, 'gauge': Gauge, 'thermometer': Thermometer, 'bell': Bell,
  'terminal': SquareTerminal, 'code': Code, 'braces': Braces, 'git-branch': GitBranch, 'github': Github,
  'workflow': Workflow, 'zap': Zap, 'bot': Bot, 'wrench': Wrench, 'settings': Settings, 'app-window': AppWindow,
  'search': Search, 'scan-search': ScanSearch, 'book-open': BookOpen, 'notebook': NotebookPen, 'newspaper': Newspaper,
  'rss': Rss, 'mail': Mail, 'message': MessageCircle, 'users': Users, 'phone': Phone, 'calendar': Calendar,
  'clock': Clock, 'shopping-cart': ShoppingCart, 'lightbulb': Lightbulb, 'palette': Palette, 'car': Car,
  'plane': Plane, 'leaf': Leaf, 'sun': Sun, 'moon': Moon,
}

export const ICON_NAMES = Object.keys(ICONS)
