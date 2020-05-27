import React from 'react';
import { BrowserRouter as Router } from 'react-router-dom';
import AppLayout from '../Components/SmartComponents/AppLayout/AppLayout';
import './app.css';

const App = () => (
    <Router>
        <AppLayout />
    </Router>
);

export default App;
