import React from 'react';
import { Toolbar, ToolbarItem, ToolbarContent } from '@patternfly/react-core';
import { Button, ButtonVariant, InputGroup, TextInput } from '@patternfly/react-core';
import Metalicity from '../ParameterizedOptions/Metalicity'
import Veq from '../ParameterizedOptions/Veq'
import Vmicro from '../ParameterizedOptions/Vmicro'
import Inclination from '../ParameterizedOptions/Inclination'
import { reverseSpec } from '../../../Utils/helper';

const ToolbarItems = ({calc, selected, handleDeleteCalculation, handleCreateCalculation}) => {
    
    const items = (
      <React.Fragment>
        <ToolbarItem>
            <Metalicity/>
        </ToolbarItem>
        <ToolbarItem>
            <Vmicro/>
        </ToolbarItem>
        <ToolbarItem>
            <Veq/>
        </ToolbarItem>
        <ToolbarItem>
            <Inclination/>
        </ToolbarItem>

        <ToolbarItem>
          <Button variant="primary" onClick={() => handleCreateCalculation(reverseSpec(selected))}>{calc ? 'Re-calculate' : 'Calculate'}</Button>
        </ToolbarItem>
        <ToolbarItem variant="separator" />
        <ToolbarItem>
          <Button isDisabled={!calc} variant="secondary" onClick={() => handleDeleteCalculation(calc.metadata.name)}>Delete</Button>
        </ToolbarItem>
      </React.Fragment>
    );

    return (
      <Toolbar id="toolbar">
        <ToolbarContent>{items}</ToolbarContent>
      </Toolbar>
    );
}

export default ToolbarItems;