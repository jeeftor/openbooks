import type { Config } from 'tailwindcss'

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts,js}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50:  '#e0ecff',
          100: '#b0c6ff',
          200: '#7e9fff',
          300: '#4c79ff',
          400: '#3366ff',
          500: '#1a4fec',
          600: '#0039e6',
          700: '#002db4',
          800: '#002082',
          900: '#001351',
        }
      },
      height: {
        dvh: '100dvh'
      }
    }
  },
  plugins: []
} satisfies Config
