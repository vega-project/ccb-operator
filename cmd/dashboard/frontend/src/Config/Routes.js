import React from "react";
import {
  Switch,
  Route
} from "react-router-dom";
import DashboardPage from '../Components/SmartComponents/Pages/Dashboard'
import ContributePage from '../Components/SmartComponents/Pages/Contribution'

const Routes = () => (
    <Switch>
        <Route path="/dashboard">
            <DashboardPage />
        </Route>

        <Route path="/how-to-contribute">
            <ContributePage />
        </Route>
        <Route component = { DashboardPage } />
    </Switch>
)
   
export default Routes;