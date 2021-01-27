import React, { useState } from 'react';
import { NavLink } from "react-router-dom";
import { Nav, NavList, NavItem } from '@patternfly/react-core';

const Navigation = () =>  {
    const [activeItem, setActiveItem] = useState(0);

     return (
        <Nav onSelect={({itemId}) => setActiveItem(itemId)} aria-label="Nav" variant="horizontal" >
          <NavList>
            <NavItem itemId={0} isActive={activeItem === 0}>
              <NavLink to="/dashboard">Dashboard</NavLink>
            </NavItem>
            <NavItem itemId={1} isActive={activeItem === 1}>
              <NavLink to="/how-to-contribute">How to contribute</NavLink>
            </NavItem>
          </NavList>
        </Nav>
    );    
}

export default Navigation;