import React, { useState } from 'react';
import { Alert, AlertActionLink, AlertActionCloseButton } from '@patternfly/react-core';
// TODO notification alerts to inform users
const Notification = (variant, message) => {
    const [isVisible, setVisibility] = useState(true);
    return (
      <React.Fragment>
        {isVisible && (
          <Alert
            variant={variant}
            title="Success alert title"
            action={<AlertActionCloseButton onClose={setVisibility(false)} />}
          >
              {message}
          </Alert>
        )}
      </React.Fragment>
    );
}

export default Notification;