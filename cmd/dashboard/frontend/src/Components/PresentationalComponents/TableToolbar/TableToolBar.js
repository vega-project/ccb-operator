import React from 'react';
import Metalicity from '../DropdownOptions/Metalicity'
import Vmicro from '../DropdownOptions/Vmicro'
import Veq from '../DropdownOptions/Veq'
import Inclination from '../DropdownOptions/Inclination'

const TableToolBar = () => (
    <div>
        <Metalicity /> 
        <Vmicro/>
        <Veq />
        <Inclination />
    </div>

);

export default TableToolBar;