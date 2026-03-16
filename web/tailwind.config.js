export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        cyber: {
          bg: '#141414', // Standard Dark Mode Background
          panel: '#1f1f1f', // Standard Panel
          primary: '#1677ff', // Standard Blue
          secondary: '#722ed1', // Standard Purple
          accent: '#faad14', // Standard Yellow
          text: '#ffffff', // White
          success: '#52c41a',
          warning: '#faad14',
          danger: '#ff4d4f',
        }
      },
      backgroundImage: {
        'cyber-gradient': 'none',
        'glass-gradient': 'none',
      },
      boxShadow: {
        'neon-blue': 'none',
        'neon-purple': 'none',
      }
    },
  },
  plugins: [],
}
