import { BrowserRouter, Route, Routes } from "react-router-dom";
import AuthProvider from "./context/AuthContext";
import AuthPage from "./pages/AuthPage";
import Home from "./pages/Home";

const App = () => {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
};

export default App;

const AppRoutes = () => {
  return (
    <div>
      <Routes>
        <Route path="/" element={<AuthPage />} />
        <Route path="/home" element={<Home />} />
      </Routes>
    </div>
  );
};
